// Package park provides the Parker primitive: the lowest-level block/wake
// mechanism that every sleeping synchronizer (mutex, semaphore, cond, latch,
// future) is ultimately built from.
//
// Two operations:
//
//	Park()   suspends the CALLING goroutine until a permit is available.
//	Unpark() makes a permit available, waking the one goroutine that owns
//	         this parker (or, if none is parked yet, leaving the permit for
//	         the next Park). Never blocks the caller.
//
// The permit is the whole point. Unpark deposits a permit; Park consumes one.
// If Unpark happens BEFORE Park, the permit is remembered, so the later Park
// returns immediately instead of sleeping. This is what prevents the
// lost-wakeup race — the gap between "a waiter decided to wait" and "the
// waiter is actually asleep" in a mutex/cond slow path, during which a waker
// may fire. Because the wakeup is stored as state (a permit) rather than a
// transient event, park and unpark are correct in any order.
//
// Asymmetric by design: the goroutine that parks is NOT the one that unparks.
// A parked goroutine cannot wake itself (it runs no code while asleep); some
// OTHER goroutine must unpark it. The minimum useful configuration is two
// goroutines — one that parks, one that unparks.
//
// Mechanism, not policy: a parker is strictly 1 goroutine <-> 1 parker. It
// knows nothing about how many waiters exist, who should wake next, or
// fairness. To manage N waiters, the layer above (mutex/semaphore/cond) keeps
// a queue of parkers and picks which one to Unpark by its own policy (FIFO,
// LIFO, or wake-all). Park/Unpark themselves only ever sleep or wake a single
// owning goroutine. This policy-free shape is exactly why one Parker can serve
// as the substrate for every higher primitive, each imposing its own policy.
package park

import _ "unsafe"

//go:linkname runtime_Semacquire sync.runtime_Semacquire
func runtime_Semacquire(s *uint32)

//go:linkname runtime_Semrelease sync.runtime_Semrelease
func runtime_Semrelease(s *uint32, handoff bool, skipframes int)

// ChanParker implements Parker on top of a capacity-1 channel. The buffer is
// the permit store: it holds at most one permit. Park receives (and blocks
// when the buffer is empty); Unpark sends, using a select/default so it
// saturates at one permit and never blocks the caller.
//
// Binary permit: multiple Unparks before a Park collapse to a single permit
// (the second send hits the default branch and is dropped). This matches the
// classic LockSupport-style parker, where unparks do not accumulate.
//
// Reusable across many park/unpark cycles. The cap-1 buffer (not an unbuffered
// channel) is required: a permit must be storable when no goroutine is
// currently parked, so an early Unpark is not lost.
type ChanParker struct {
	park chan struct{}
}

func NewChanParker() *ChanParker {
	return &ChanParker{
		park: make(chan struct{}, 1),
	}
}

func (p *ChanParker) Park() {
	<-p.park
}

func (p *ChanParker) Unpark() {
	select {
	case p.park <- struct{}{}:
	default:
	}
}

// Parker implements Parker on top of the Go runtime semaphore, reached through
// a linkname into sync.runtime_Semacquire / runtime_Semrelease. The sema's
// counter is the permit store, so a parked goroutine is parked by the runtime
// itself (gopark): it yields its P and consumes no CPU while waiting — the
// same machinery sync.Mutex and sync.WaitGroup use on their slow paths.
//
// Caveat vs ChanParker: the runtime sema COUNTS rather than saturating. N
// Unparks before a Park leave N permits, so the next N Parks all return
// without sleeping. For a strict binary-permit (LockSupport) contract, guard
// the sema with an atomic flag so repeated Unparks collapse to one.
//
// Zero-value usable (a fresh Parker has no permit, so the first Park sleeps).
// Must not be copied after first use: the runtime keys the wait queue on
// &p.sema, so the address must stay stable.
type Parker struct {
	sema uint32
}

func (p *Parker) Park() {
	runtime_Semacquire(&p.sema)
}

func (p *Parker) Unpark() {
	runtime_Semrelease(&p.sema, false, 0)
}
