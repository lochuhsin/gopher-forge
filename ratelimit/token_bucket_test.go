package ratelimit

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// ---------- constructor & Allow ----------

func TestNewTokenBucket_StartsFull(t *testing.T) {
	tb := NewTokenBucket(5, 0) // rate=0 -> never refills
	for i := range 5 {
		if !tb.Allow() {
			t.Fatalf("Allow() #%d: want true, got false", i+1)
		}
	}
	if tb.Allow() {
		t.Error("Allow() #6: bucket should be empty")
	}
}

// ---------- AllowN ----------

func TestAllowN(t *testing.T) {
	t.Run("n=0 always succeeds even when empty", func(t *testing.T) {
		tb := NewTokenBucket(5, 0)
		for range 5 {
			tb.Allow()
		}
		if !tb.AllowN(0) {
			t.Error("AllowN(0) on empty bucket: want true, got false")
		}
	})

	t.Run("n>cap returns false", func(t *testing.T) {
		tb := NewTokenBucket(5, 0)
		if tb.AllowN(6) {
			t.Error("AllowN(6) with cap=5: want false, got true")
		}
	})

	t.Run("n==cap on full bucket succeeds and drains", func(t *testing.T) {
		tb := NewTokenBucket(5, 0)
		if !tb.AllowN(5) {
			t.Error("AllowN(5) on full cap=5: want true, got false")
		}
		if tb.Allow() {
			t.Error("expected empty after AllowN(cap)")
		}
	})

	t.Run("consumes exactly n tokens", func(t *testing.T) {
		tb := NewTokenBucket(10, 0)
		if !tb.AllowN(7) {
			t.Fatal("AllowN(7): want true, got false")
		}
		remaining := 0
		for tb.Allow() {
			remaining++
		}
		if remaining != 3 {
			t.Errorf("remaining tokens: want 3, got %d", remaining)
		}
	})

	t.Run("insufficient tokens returns false without consuming", func(t *testing.T) {
		tb := NewTokenBucket(10, 0)
		tb.AllowN(8) // 2 left
		if tb.AllowN(3) {
			t.Error("AllowN(3) with 2 left: want false, got true")
		}
		if !tb.AllowN(2) {
			t.Error("AllowN(2) after failed AllowN(3): want true, got false")
		}
	})
}

// ---------- refill over time ----------
// synctest gives us a deterministic fake clock: time.Sleep advances the bubble's
// virtual time without real waiting, so refill math is testable without flakiness.

func TestAllow_RefillsOverTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// cap=10, rate=100/s -> 1 token / 10ms
		tb := NewTokenBucket(10, 100)
		for range 10 {
			tb.Allow()
		}
		if tb.Allow() {
			t.Fatal("expected empty bucket")
		}

		time.Sleep(50 * time.Millisecond) // 5 tokens worth

		count := 0
		for tb.Allow() {
			count++
		}
		if count != 5 {
			t.Errorf("refill after 50ms @ 100/s: want 5, got %d", count)
		}
	})
}

func TestRefill_CapsAtMax(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tb := NewTokenBucket(5, 1000)
		for range 5 {
			tb.Allow()
		}
		time.Sleep(1 * time.Second) // would refill 1000 if uncapped

		count := 0
		for tb.Allow() {
			count++
		}
		if count != 5 {
			t.Errorf("refill capped at cap: want 5, got %d", count)
		}
	})
}

// ---------- Wait ----------

func TestWait_ReturnsImmediatelyWhenAvailable(t *testing.T) {
	tb := NewTokenBucket(2, 0)
	done := make(chan struct{})
	go func() {
		tb.Wait(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Wait() did not return within 50ms despite token available")
	}
}

func TestWait_BlocksUntilRefill(t *testing.T) {
	// cap=1, rate=200/s -> 5ms / token
	tb := NewTokenBucket(1, 200)
	tb.Allow()

	done := make(chan time.Duration, 1)
	start := time.Now()
	go func() {
		tb.Wait(context.Background())
		done <- time.Since(start)
	}()

	select {
	case elapsed := <-done:
		if elapsed < 2*time.Millisecond {
			t.Errorf("Wait returned too quickly: %v", elapsed)
		}
		if elapsed > 200*time.Millisecond {
			t.Errorf("Wait took too long: %v", elapsed)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Wait did not return within 500ms")
	}
}

func TestWait_RespectsContextCancellation(t *testing.T) {
	tb := NewTokenBucket(1, 0)
	tb.Allow() // drain so Wait would block forever without ctx

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		tb.Wait(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Wait() did not return after context cancellation")
	}
}

// ---------- WaitN ----------

func TestWaitN_ReturnsErrorWhenNExceedsCap(t *testing.T) {
	tb := NewTokenBucket(5, 100)
	if err := tb.WaitN(context.Background(), 6); err == nil {
		t.Error("WaitN(6) with cap=5: want error, got nil")
	}
}

func TestWaitN_HappyPath(t *testing.T) {
	tb := NewTokenBucket(5, 0)
	if err := tb.WaitN(context.Background(), 3); err != nil {
		t.Fatalf("WaitN(3) on full bucket: want nil, got %v", err)
	}
	if !tb.AllowN(2) {
		t.Error("expected 2 tokens remaining")
	}
	if tb.Allow() {
		t.Error("expected bucket empty")
	}
}

func TestWaitN_BlocksUntilEnoughTokens(t *testing.T) {
	// cap=5, rate=500/s -> 2ms / token; waiting for 3 -> ~6ms
	tb := NewTokenBucket(5, 500)
	tb.AllowN(5)

	type result struct {
		elapsed time.Duration
		err     error
	}
	done := make(chan result, 1)
	start := time.Now()
	go func() {
		err := tb.WaitN(context.Background(), 3)
		done <- result{time.Since(start), err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatal(r.err)
		}
		if r.elapsed < 2*time.Millisecond {
			t.Errorf("WaitN returned too quickly: %v", r.elapsed)
		}
		if r.elapsed > 200*time.Millisecond {
			t.Errorf("WaitN took too long: %v", r.elapsed)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("WaitN did not return within 500ms")
	}
}

func TestWaitN_RespectsContextCancellation(t *testing.T) {
	tb := NewTokenBucket(5, 0)
	tb.AllowN(5)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- tb.WaitN(ctx, 3)
	}()

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("want context.Canceled, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("WaitN() did not return after context cancellation")
	}
}

// ---------- concurrency ----------
// Run with `go test -race`. rate=0 means no refill, so total successes must
// equal exactly cap regardless of scheduling.

func TestAllow_ConcurrentSafety(t *testing.T) {
	const capacity = 100
	tb := NewTokenBucket(capacity, 0)

	var allowed atomic.Uint64
	var wg sync.WaitGroup
	for range 1000 {
		wg.Go(func() {
			if tb.Allow() {
				allowed.Add(1)
			}
		})
	}
	wg.Wait()

	if got := allowed.Load(); got != capacity {
		t.Errorf("concurrent Allow: want exactly %d successes, got %d", capacity, got)
	}
}
