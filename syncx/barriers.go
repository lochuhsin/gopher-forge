package syncx

import (
	"sync/atomic"
)

// CountingBarrier is a one-shot centralized barrier: N goroutines each
// atomically decrement a shared seat counter and spin until it reaches zero.
// The last arriver implicitly releases everyone by transitioning the counter
// to 0.
//
// Pros:
//   - Extremely simple: one atomic counter, one fetch-and-add to arrive, one
//     load in the spin loop.
//   - Zero allocations after construction; tiny memory footprint.
//   - Easy to reason about — no sense bits, no epochs, no tree structure.
//
// Cons:
//   - One-shot only. Once seats hits 0 there is no safe way to reuse the
//     barrier: resetting while late arrivers are still spinning would corrupt
//     the protocol. See SenseReversingBarrier for the reusable variant.
//   - Cache-line bouncing under contention. Every waiter spins on the same
//     `seats` variable, so each decrement invalidates the cache line on all
//     N-1 spinning cores — the same O(N^2) coherence traffic described on
//     TicketLock.
//   - Busy-wait burns CPU while waiters block.
type CountingBarrier struct {
	seats *atomic.Int32
}

func NewCountingBarrier(n int32) *CountingBarrier {
	if n <= 0 {
		panic("barrier size must be positive")
	}
	seats := atomic.Int32{}
	seats.Store(n)
	return &CountingBarrier{
		seats: &seats,
	}
}

func (b *CountingBarrier) Wait() {
	b.seats.Add(-1)
	for b.seats.Load() > 0 {
		// spin
	}
}

// SenseReversingBarrier is a reusable centralized barrier. Each Wait() captures
// the current `release` epoch on entry, then decrements `seats`. The last
// arriver resets `seats` for the next phase and bumps `release` (the "sense"),
// which signals everyone else to exit their spin.
//
// The reset-before-publish ordering is essential:
// state preparation (seats reset) → publication (release++) → observation
// (waiters spinning on release). The publisher must finish preparing state
// before publishing, and observers must already be spinning before the publish
// happens. The atomic RMW on `release` provides the release/acquire ordering
// that makes this safe.
//
// Pros:
//   - Reusable across phases. The release counter doubles as an epoch that
//     naturally distinguishes "the barrier I'm waiting on now" from "the next
//     one", with no extra coordination needed.
//   - The last arriver handles all the reset work, so there is no separate
//     leader/coordinator role.
//   - Same low operation count as CountingBarrier on the fast path: one add
//     and one load in the spin.
//
// Cons:
//   - Still centralized: all N waiters spin on the same `release` variable, so
//     the cache-line bouncing problem is identical to CountingBarrier and
//     TicketLock — O(N^2) coherence traffic to drain a phase.
//   - Busy-wait burns CPU.
//   - The last arriver pays a slightly higher cost (seats reset + release
//     increment) than the others, though this is amortized across the phase.
type SenseReversingBarrier struct {
	release *atomic.Uint32
	seats   *atomic.Int32
	count   int32
}

func NewSenseReversingBarrier(n int32) *SenseReversingBarrier {
	if n <= 0 {
		panic("barrier size must be positive")
	}
	arrival := atomic.Int32{}
	arrival.Store(n)
	return &SenseReversingBarrier{
		release: &atomic.Uint32{},
		seats:   &arrival,
		count:   n,
	}
}

/*
The key point is that the path of the last goroutine is different than
normal one.

「state preparation → publication → observation」這三件事的時間順序不能亂。
Publisher 必須在準備好 state 之後才發 → Observer 必須在 publish 之前就開始觀察。
*/
func (s *SenseReversingBarrier) Wait() {
	release := s.release.Load()

	if s.seats.Add(-1) == 0 {
		s.seats.Store(s.count)
		s.release.Add(1) //release-after-prepare ordering
		return
	}

	for s.release.Load() == release {
		// Spin
	}
}

// CombiningTreeBarrier is a tree-structured barrier that distributes arrival
// counting across a tree of small sub-barriers. Goroutines arrive at leaves
// and the "I'm done" signal propagates up to the root; the release signal then
// propagates back down. With fan-in k, the tree has depth O(log_k N), and at
// each internal node only k contenders ever touch its counter.
//
// Pros:
//   - Distributes contention. Instead of all N goroutines hammering one
//     counter, each internal node sees only a small fan-in (typically 2-4),
//     so per-phase coherence traffic drops from O(N^2) to O(N log N).
//   - Scales much better than centralized barriers on many-core machines —
//     this is the standard fix once N grows past a single socket.
//   - Spinning is localized to per-node flags, so cache-line bouncing is
//     bounded by the fan-in, not by N. Conceptually the same win MCSLock
//     gives over TicketLock, applied to barriers.
//
// Cons:
//   - Latency per phase is O(log N): the arrival wave must climb to the root
//     and the release wave must descend back down, instead of a single
//     decrement-and-spin.
//   - More complex: tree construction, per-goroutine routing to a leaf, and
//     careful handling of arrival/release state at each node.
//   - Higher constant cost at low N — for small thread counts, a centralized
//     barrier wins.
//   - Memory overhead proportional to N (one node per group, ideally padded
//     to avoid false sharing).
type CombiningTreeBarrier struct{}

func NewCombiningTreeBarrier(n int32) *CombiningTreeBarrier {
	return &CombiningTreeBarrier{}
}

func (c *CombiningTreeBarrier) Wait() {
}

type StaticTreeBarrier struct{}

func NewStaticTreeBarrier(n int32) *StaticTreeBarrier {
	return &StaticTreeBarrier{}
}

func (s *StaticTreeBarrier) Wait() {

}

type TournamentBarrier struct {
}

func NewTournamentBarrier(n int32) *TournamentBarrier {
	return &TournamentBarrier{}
}

func (t *TournamentBarrier) Wait() {

}

type DisseminationBarrier struct{}

func NewDisseminationBarrier(n int32) *DisseminationBarrier {
	return &DisseminationBarrier{}
}

func (d *DisseminationBarrier) Wait() {

}

type ButterflyBarrier struct{}

func NewButterflyBarrier(n int32) *ButterflyBarrier {
	return &ButterflyBarrier{}
}

func (b *ButterflyBarrier) Wait() {

}
