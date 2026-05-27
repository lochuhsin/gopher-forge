package memory

import (
	"sync"
	"testing"
)

// AtomicCell stores a *copy* of the value it is given (Store takes T by
// value and publishes &v), and Load returns the zero value of T until
// the first Store. These tests pin down that contract across basic
// types, the zero-value case, pointer element types, copy semantics,
// and concurrent access (run with -race).

func TestAtomicCellBasicTypes(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		var c AtomicCell[int]
		c.Store(42)
		if got := c.Load(); got != 42 {
			t.Errorf("Load = %d, want 42", got)
		}
	})

	t.Run("string", func(t *testing.T) {
		var c AtomicCell[string]
		c.Store("hello")
		if got := c.Load(); got != "hello" {
			t.Errorf("Load = %q, want %q", got, "hello")
		}
	})

	t.Run("float64", func(t *testing.T) {
		var c AtomicCell[float64]
		c.Store(3.14)
		if got := c.Load(); got != 3.14 {
			t.Errorf("Load = %v, want 3.14", got)
		}
	})

	t.Run("bool", func(t *testing.T) {
		var c AtomicCell[bool]
		c.Store(true)
		if got := c.Load(); got != true {
			t.Errorf("Load = %v, want true", got)
		}
	})
}

// Load before any Store must return T's zero value, not panic or return
// garbage — the nil-pointer branch in Load.
func TestAtomicCellZeroValue(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		var c AtomicCell[int]
		if got := c.Load(); got != 0 {
			t.Errorf("Load on empty = %d, want 0", got)
		}
	})

	t.Run("string", func(t *testing.T) {
		var c AtomicCell[string]
		if got := c.Load(); got != "" {
			t.Errorf("Load on empty = %q, want \"\"", got)
		}
	})

	t.Run("struct", func(t *testing.T) {
		type point struct{ X, Y int }
		var c AtomicCell[point]
		if got := c.Load(); got != (point{}) {
			t.Errorf("Load on empty = %+v, want zero struct", got)
		}
	})

	t.Run("pointer", func(t *testing.T) {
		var c AtomicCell[*int]
		if got := c.Load(); got != nil {
			t.Errorf("Load on empty = %v, want nil", got)
		}
	})

	// A zero-valued Store must round-trip: it is indistinguishable from
	// the never-stored case at the value level, but must not panic.
	t.Run("store zero int", func(t *testing.T) {
		var c AtomicCell[int]
		c.Store(0)
		if got := c.Load(); got != 0 {
			t.Errorf("Load after Store(0) = %d, want 0", got)
		}
	})
}

func TestAtomicCellPointerType(t *testing.T) {
	t.Run("store non-nil", func(t *testing.T) {
		var c AtomicCell[*int]
		v := 7
		c.Store(&v)
		got := c.Load()
		if got == nil {
			t.Fatal("Load = nil, want non-nil pointer")
		}
		if *got != 7 {
			t.Errorf("*Load = %d, want 7", *got)
		}
		if got != &v {
			t.Error("Load returned a different pointer than stored; element copy is the pointer itself, so identity must be preserved")
		}
	})

	// Storing a nil *int is legal and must read back as nil rather than
	// dereference-panic: the cell holds &v where v is a nil *int.
	t.Run("store nil", func(t *testing.T) {
		var c AtomicCell[*int]
		c.Store(nil)
		if got := c.Load(); got != nil {
			t.Errorf("Load after Store(nil) = %v, want nil", got)
		}
	})
}

// Store copies the value, so mutating the caller's original after Store
// must not change what the cell holds.
func TestAtomicCellStoreCopies(t *testing.T) {
	type point struct{ X, Y int }
	var c AtomicCell[point]

	p := point{X: 1, Y: 2}
	c.Store(p)
	p.X, p.Y = 99, 99 // mutate caller's copy after storing

	if got := c.Load(); got != (point{X: 1, Y: 2}) {
		t.Errorf("Load = %+v, want {X:1 Y:2}; Store must snapshot by value", got)
	}
}

func TestAtomicCellOverwrite(t *testing.T) {
	var c AtomicCell[int]
	for i := range 100 {
		c.Store(i)
		if got := c.Load(); got != i {
			t.Fatalf("after Store(%d): Load = %d", i, got)
		}
	}
}

// Each Load must observe a fully published value, never a torn one.
// Writers only ever store pairs with a == b; a reader that sees a != b
// witnessed a torn read. Run with -race to also catch the data race
// directly.
func TestAtomicCellConcurrentNoTear(t *testing.T) {
	type pair struct{ a, b int }
	var c AtomicCell[pair]
	c.Store(pair{})

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
				c.Store(pair{a: v, b: v})
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
					if p := c.Load(); p.a != p.b {
						t.Errorf("torn read: %+v (a != b)", p)
						return
					}
				}
			}
		}()
	}

	// Once every writer is done, tell the readers to drain and exit.
	writersWg.Wait()
	close(stop)
	readersWg.Wait()
}