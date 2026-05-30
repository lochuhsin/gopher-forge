package memory

import (
	"sync"
	"testing"
)

// ReadyFlag is a one-shot publication primitive: Publish stores v into
// r.data and then flips an atomic.Bool from false to true; Observe reads
// the flag and, if set, returns the data. These tests pin down the
// contract across basic types, the never-published case, pointer
// elements, copy semantics, the double-Publish panic, and the
// publication-safety guarantee (run with -race).

func TestReadyFlagBasicTypes(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		var r ReadyFlag[int]
		r.Publish(42)
		got, ok := r.Observe()
		if !ok {
			t.Fatal("Observe ok = false after Publish, want true")
		}
		if got != 42 {
			t.Errorf("Observe = %d, want 42", got)
		}
	})

	t.Run("string", func(t *testing.T) {
		var r ReadyFlag[string]
		r.Publish("hello")
		got, ok := r.Observe()
		if !ok {
			t.Fatal("Observe ok = false after Publish, want true")
		}
		if got != "hello" {
			t.Errorf("Observe = %q, want %q", got, "hello")
		}
	})

	t.Run("float64", func(t *testing.T) {
		var r ReadyFlag[float64]
		r.Publish(3.14)
		got, ok := r.Observe()
		if !ok {
			t.Fatal("Observe ok = false after Publish, want true")
		}
		if got != 3.14 {
			t.Errorf("Observe = %v, want 3.14", got)
		}
	})

	t.Run("bool", func(t *testing.T) {
		var r ReadyFlag[bool]
		r.Publish(true)
		got, ok := r.Observe()
		if !ok {
			t.Fatal("Observe ok = false after Publish, want true")
		}
		if got != true {
			t.Errorf("Observe = %v, want true", got)
		}
	})
}

// Observe before any Publish must return T's zero value and false, not
// panic or expose r.data's uninitialized memory. Critically, ok=false
// must be the only signal a caller relies on: the returned T is
// meaningless and may equal a legal published value.
func TestReadyFlagObserveBeforePublish(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		var r ReadyFlag[int]
		got, ok := r.Observe()
		if ok {
			t.Errorf("Observe ok = true before Publish, want false")
		}
		if got != 0 {
			t.Errorf("Observe = %d, want 0", got)
		}
	})

	t.Run("string", func(t *testing.T) {
		var r ReadyFlag[string]
		got, ok := r.Observe()
		if ok {
			t.Errorf("Observe ok = true before Publish, want false")
		}
		if got != "" {
			t.Errorf("Observe = %q, want \"\"", got)
		}
	})

	t.Run("struct", func(t *testing.T) {
		type point struct{ X, Y int }
		var r ReadyFlag[point]
		got, ok := r.Observe()
		if ok {
			t.Errorf("Observe ok = true before Publish, want false")
		}
		if got != (point{}) {
			t.Errorf("Observe = %+v, want zero struct", got)
		}
	})

	t.Run("pointer", func(t *testing.T) {
		var r ReadyFlag[*int]
		got, ok := r.Observe()
		if ok {
			t.Errorf("Observe ok = true before Publish, want false")
		}
		if got != nil {
			t.Errorf("Observe = %v, want nil", got)
		}
	})

	// Publishing the zero value must still flip ready: ok=true is the
	// only thing that distinguishes "published zero" from "never
	// published".
	t.Run("publish zero int", func(t *testing.T) {
		var r ReadyFlag[int]
		r.Publish(0)
		got, ok := r.Observe()
		if !ok {
			t.Fatal("Observe ok = false after Publish(0), want true")
		}
		if got != 0 {
			t.Errorf("Observe = %d, want 0", got)
		}
	})
}

func TestReadyFlagPointerType(t *testing.T) {
	t.Run("publish non-nil", func(t *testing.T) {
		var r ReadyFlag[*int]
		v := 7
		r.Publish(&v)
		got, ok := r.Observe()
		if !ok {
			t.Fatal("Observe ok = false after Publish, want true")
		}
		if got == nil {
			t.Fatal("Observe = nil, want non-nil pointer")
		}
		if *got != 7 {
			t.Errorf("*Observe = %d, want 7", *got)
		}
		if got != &v {
			t.Error("Observe returned a different pointer than published; T is the pointer itself, so identity must be preserved")
		}
	})

	// Publishing a nil *int is legal and must read back as nil with
	// ok=true — the flag, not the pointer, carries the publication
	// signal.
	t.Run("publish nil", func(t *testing.T) {
		var r ReadyFlag[*int]
		r.Publish(nil)
		got, ok := r.Observe()
		if !ok {
			t.Fatal("Observe ok = false after Publish(nil), want true")
		}
		if got != nil {
			t.Errorf("Observe = %v, want nil", got)
		}
	})
}

// Publish copies the value into r.data, so mutating the caller's
// original after Publish must not change what Observe returns.
func TestReadyFlagPublishCopies(t *testing.T) {
	type point struct{ X, Y int }
	var r ReadyFlag[point]

	p := point{X: 1, Y: 2}
	r.Publish(p)
	p.X, p.Y = 99, 99 // mutate caller's copy after publishing

	got, ok := r.Observe()
	if !ok {
		t.Fatal("Observe ok = false after Publish, want true")
	}
	if got != (point{X: 1, Y: 2}) {
		t.Errorf("Observe = %+v, want {X:1 Y:2}; Publish must snapshot by value", got)
	}
}

// Calling Publish twice violates the one-shot contract and must panic;
// the CAS from false->true is the linearization point that detects it.
func TestReadyFlagDoublePublishPanics(t *testing.T) {
	var r ReadyFlag[int]
	r.Publish(1)

	defer func() {
		if rec := recover(); rec == nil {
			t.Error("second Publish did not panic")
		}
	}()
	r.Publish(2)
}

// Repeated Observe calls after a single Publish must keep returning the
// same value with ok=true — Observe is read-only and idempotent.
func TestReadyFlagObserveIdempotent(t *testing.T) {
	var r ReadyFlag[int]
	r.Publish(123)
	for i := range 100 {
		got, ok := r.Observe()
		if !ok {
			t.Fatalf("iter %d: Observe ok = false, want true", i)
		}
		if got != 123 {
			t.Fatalf("iter %d: Observe = %d, want 123", i, got)
		}
	}
}

// Publication safety: any Observe that sees ok=true must see the fully
// written struct, never a torn value. Writers only ever publish pairs
// with a == b; a reader that sees a != b witnessed a tear across the
// r.data write and the ready.Store. Run with -race to also catch the
// data race directly.
//
// The test launches many observers that spin on Observe before
// Publish is called, so most of their Observe calls happen
// concurrently with the publishing goroutine.
func TestReadyFlagConcurrentPublication(t *testing.T) {
	type pair struct{ a, b int }

	const trials = 200
	for trial := range trials {
		var r ReadyFlag[pair]

		const observers = 8
		var wg sync.WaitGroup
		start := make(chan struct{})

		for range observers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				for {
					v, ok := r.Observe()
					if ok {
						if v.a != v.b {
							t.Errorf("trial %d: torn read: %+v (a != b)", trial, v)
						}
						return
					}
				}
			}()
		}

		close(start)
		r.Publish(pair{a: trial, b: trial})
		wg.Wait()
	}
}

// PublishedPointer publishes a *T atomically. Unlike ReadyFlag it is
// not one-shot — callers can swap in a fresh pointer at any time —
// and the "not yet published" signal is a nil return rather than a
// separate bool. These tests pin down the initial-nil case, round-
// trip identity, explicit nil publication, repeated republish, and
// the publication-safety guarantee that any observed non-nil pointer
// reaches a fully initialized pointee (run with -race).

// Observe before any Publish must return nil — there is no published
// pointer yet, and Observe must not invent one.
func TestPublishedPointerObserveBeforePublish(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		var pp PublishedPointer[int]
		if got := pp.Observe(); got != nil {
			t.Errorf("Observe before Publish = %v, want nil", got)
		}
	})

	t.Run("struct", func(t *testing.T) {
		type point struct{ X, Y int }
		var pp PublishedPointer[point]
		if got := pp.Observe(); got != nil {
			t.Errorf("Observe before Publish = %v, want nil", got)
		}
	})
}

// PublishedPointer is a pointer publication primitive, not a value
// copy: Observe must return the exact pointer that was Published, not
// an alias allocated by the cell.
func TestPublishedPointerIdentity(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		var pp PublishedPointer[int]
		v := 42
		pp.Publish(&v)
		got := pp.Observe()
		if got != &v {
			t.Error("Observe returned a different pointer than Published; PublishedPointer must not copy")
		}
		if *got != 42 {
			t.Errorf("*Observe = %d, want 42", *got)
		}
	})

	t.Run("struct", func(t *testing.T) {
		type point struct{ X, Y int }
		var pp PublishedPointer[point]
		p := point{X: 3, Y: 4}
		pp.Publish(&p)
		got := pp.Observe()
		if got != &p {
			t.Error("Observe returned a different pointer than Published; PublishedPointer must not copy")
		}
		if *got != p {
			t.Errorf("*Observe = %+v, want %+v", *got, p)
		}
	})
}

// Publishing nil is legal and must round-trip — useful for
// "unpublish" or sentinel transitions. It must also work as the
// first Publish, not just after a non-nil one.
func TestPublishedPointerPublishNil(t *testing.T) {
	t.Run("nil first", func(t *testing.T) {
		var pp PublishedPointer[int]
		pp.Publish(nil)
		if got := pp.Observe(); got != nil {
			t.Errorf("Observe after Publish(nil) = %v, want nil", got)
		}
	})

	t.Run("nil after non-nil", func(t *testing.T) {
		var pp PublishedPointer[int]
		v := 1
		pp.Publish(&v)
		pp.Publish(nil)
		if got := pp.Observe(); got != nil {
			t.Errorf("Observe after Publish(nil) = %v, want nil", got)
		}
	})
}

// PublishedPointer is not one-shot: each Publish replaces the
// previously published pointer, and Observe always sees the latest.
// The caller invariant — do not mutate a previously published *T —
// is enforced socially, not by the type; this test honours it by
// always publishing a fresh allocation.
func TestPublishedPointerRepublish(t *testing.T) {
	var pp PublishedPointer[int]
	for i := range 100 {
		v := i
		pp.Publish(&v)
		got := pp.Observe()
		if got == nil {
			t.Fatalf("after Publish(&%d): Observe = nil", i)
		}
		if *got != i {
			t.Fatalf("after Publish(&%d): *Observe = %d", i, *got)
		}
	}
}

// Publication safety: any Observe that returns non-nil must point at
// a fully-constructed pair. Writers only publish &pair{a: v, b: v},
// so a reader that ever sees p.a != p.b witnessed a tear between the
// pointee's field writes and the atomic.Pointer.Store. Run with
// -race to also catch the data race directly.
//
// Every Publish allocates a fresh *pair, so the "do not mutate after
// Publish" caller invariant is upheld without coordination.
func TestPublishedPointerConcurrentPublication(t *testing.T) {
	type pair struct{ a, b int }
	var pp PublishedPointer[pair]

	const (
		writers = 4
		readers = 4
		ops     = 50_000
	)

	stop := make(chan struct{})
	var writersWg, readersWg sync.WaitGroup

	for w := range writers {
		writersWg.Add(1)
		go func() {
			defer writersWg.Done()
			for i := range ops {
				v := w*ops + i
				pp.Publish(&pair{a: v, b: v})
			}
		}()
	}

	for range readers {
		readersWg.Add(1)
		go func() {
			defer readersWg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					if p := pp.Observe(); p != nil && p.a != p.b {
						t.Errorf("torn read: %+v (a != b)", *p)
						return
					}
				}
			}
		}()
	}

	writersWg.Wait()
	close(stop)
	readersWg.Wait()
}
