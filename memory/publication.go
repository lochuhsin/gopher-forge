package memory

import "sync/atomic"

// ReadyFlag[T] is a one-shot publication primitive used to demonstrate
// the acquire/release pattern in Go. A single Publish call hands a T
// value off to any number of Observe callers via an atomic ready flag.
//
// Substrate: sync/atomic.Bool over an ordinary T field.
// Linearization point: the Store(true) call in Publish.
// Progress: wait-free on both sides (single atomic op, no loop).
// Safety: Publish must be called at most once. Calling it twice races
// with any in-flight Observe and is a usage error.
type ReadyFlag[T any] struct {
	data  T
	ready atomic.Bool
}

func (r *ReadyFlag[T]) Publish(v T) {
	r.data = v
	if !r.ready.CompareAndSwap(false, true) {
		panic("Publish is called more than once")
	}
}
func (r *ReadyFlag[T]) Observe() (T, bool) {
	if r.ready.Load() {
		return r.data, true
	}
	return *new(T), false
}

// PublishedPointer[T] publishes a *T such that any goroutine that
// observes a non-nil pointer is guaranteed to see the struct it
// points to fully initialized.
//
// Substrate: sync/atomic.Pointer[T].
// Linearization point: the Store call in Publish.
// Progress: wait-free on both sides (single atomic op, no loop).
//
// Caller invariant: the *T passed to Publish MUST NOT be mutated by
// anyone after the Publish call returns. To update, build a new *T
// and Publish that — never reach into a previously published value.
// Violating this is a data race that -race will catch.
type PublishedPointer[T any] struct {
	p atomic.Pointer[T]
}

func (pp *PublishedPointer[T]) Publish(v *T) {
	pp.p.Store(v)
}
func (pp *PublishedPointer[T]) Observe() *T {
	return pp.p.Load()
}

// CacheLineSize is the conservative cache-line granule we pad against.
// Apple Silicon uses 128-byte coherency lines; many x86 CPUs use 64.
// 128 is the safe over-pad: it covers both at the cost of memory.
const CacheLineSize = 128

// UnpaddedCounters places two counters adjacently so they share one
// cache line on every commodity CPU. Concurrent writers to a and b
// thrash MESI on every increment — the textbook false-sharing demo.
type UnpaddedCounters struct {
	a atomic.Uint64
	b atomic.Uint64
}

func (c *UnpaddedCounters) IncA() {
	c.a.Add(1)
}
func (c *UnpaddedCounters) IncB() {
	c.b.Add(1)
}
func (c *UnpaddedCounters) LoadA() uint64 {
	return c.a.Load()
}
func (c *UnpaddedCounters) LoadB() uint64 {
	return c.b.Load()
}

// PaddedCounters separates a and b by one cache line so two writers
// updating them in parallel do not invalidate each other's cache.
type PaddedCounters struct {
	a atomic.Uint64
	_ [CacheLineSize - 8]byte
	b atomic.Uint64
	_ [CacheLineSize - 8]byte
}

func (c *PaddedCounters) IncA() {
	c.a.Add(1)
}
func (c *PaddedCounters) IncB() {
	c.b.Add(1)
}
func (c *PaddedCounters) LoadA() uint64 {
	return c.a.Load()
}
func (c *PaddedCounters) LoadB() uint64 {
	return c.b.Load()
}
