package ratelimit

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrExceedsCapacity = errors.New("ratelimit: requested tokens exceed bucket capacity")

// TokenBucket is a thread-safe token-bucket rate limiter with lazy refill.
//
// Tokens accumulate continuously at 1 token per `interval` up to `capacity`.
// Each Allow / Wait call recomputes accumulated tokens from elapsed time
// since the last refill, so there is no background goroutine or ticker.
//
// Pros:
//   - Zero allocation on the hot path; no background goroutine.
//   - Smooth admission at any rate up to ~1e9 tokens/sec (limited by the ns
//     resolution of time.Now).
//   - Idle buckets cost nothing; refill is lazy and on-demand.
//   - Sub-token elapsed time is preserved across refills, so low-rate
//     buckets eventually accumulate regardless of polling frequency.
//
// Cons:
//   - A single sync.Mutex serializes all paths. Under extreme contention
//     (>1M ops/sec on one bucket) prefer a lock-free reservation-cursor
//     variant.
//   - No FIFO fairness across Wait callers; sync.Mutex is not strictly
//     ordered, so under contention waiters may be served out of arrival
//     order.
//   - (tokens, last) is a linked invariant — both must be updated under the
//     same mutex. A single atomic.Int64 cannot replace the mutex without
//     redesigning the state representation.
//
// Mental model:
//
//	rate     = tokens/sec accepted by user (e.g. 100)
//	interval = time per token (e.g. 10ms for rate=100)
//	last     = timestamp of the most recent fully-consumed refill point
//	tokens   = current bucket level, in [0, capacity]
//
//	refillLocked:
//	    elapsed = now - last
//	    added   = elapsed / interval                  (integer division)
//	    tokens  = min(capacity, tokens + added)
//	    last   += added * interval                    (preserve remainder)
type TokenBucket struct {
	capacity uint64
	interval time.Duration
	last     time.Time
	tokens   uint64
	mu       sync.Mutex
}

func NewTokenBucket(capacity uint64, ratePerSec uint64) *TokenBucket {
	var interval time.Duration
	if ratePerSec > 0 {
		interval = time.Second / time.Duration(ratePerSec)
	}
	return &TokenBucket{
		capacity: capacity,
		interval: interval,
		last:     time.Now(),
		tokens:   capacity,
	}
}

func (t *TokenBucket) Allow() bool {
	return t.AllowN(1)
}

func (t *TokenBucket) AllowN(n uint64) bool {
	if n > t.capacity {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.refillLocked()
	if t.tokens < n {
		return false
	}
	t.tokens -= n
	return true
}

func (t *TokenBucket) Wait(ctx context.Context) error {
	return t.WaitN(ctx, 1)
}

func (t *TokenBucket) WaitN(ctx context.Context, n uint64) error {
	if n > t.capacity {
		return ErrExceedsCapacity
	}
	for {
		t.mu.Lock()
		t.refillLocked()
		if t.tokens >= n {
			t.tokens -= n
			t.mu.Unlock()
			return nil
		}
		if t.interval == 0 {
			t.mu.Unlock()
			<-ctx.Done()
			return ctx.Err()
		}
		needed := n - t.tokens
		waitFor := time.Duration(needed) * t.interval
		t.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitFor):
		}
	}
}

func (t *TokenBucket) refillLocked() {
	if t.interval == 0 {
		return
	}
	elapsed := time.Since(t.last)
	added := uint64(elapsed / t.interval)
	t.tokens = min(t.capacity, t.tokens+added)
	t.last = t.last.Add(time.Duration(added) * t.interval)
}
