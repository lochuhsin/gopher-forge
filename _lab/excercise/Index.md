# excercise Index

> Learning goal: solve classic concurrency puzzles with existing primitives, compare approaches, and expose deadlock, starvation, fairness, and rendezvous pitfalls.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Lab Notes

- This lab should solve puzzles with existing primitives only. Do not introduce new synchronization primitives here.
- Each exercise should ideally have two or three versions, such as mutex/cond, channels, and semaphores, then compare fairness and failure modes.
- Recommended build order from the merged TODO: Print in Order, Dining Philosophers, Readers-Writers, Sleeping Barber, Cigarette Smokers, H2O, Roller Coaster, Santa Claus, river/invariant puzzles, then larger free-form exercises.
- Dependencies: imports `syncx/` and the standard library; larger exercises may also use `queue/`, `mapx/`, `ratelimit/`, and `actor/`.
- Career signal: useful for whiteboard concurrency and for explaining deadlock, starvation, fairness, and group rendezvous.
- Scope rule: exercises should reuse package primitives and compare multiple solutions; avoid puzzle variants that only test language syntax.

## Reference Trail and Go Boundary

- Exercise reference: The Little Book of Semaphores (`https://greenteapress.com/wp/semaphores/`) and the standard Go concurrency examples are the right source style here.
- Mental model: exercises are drills for recognizing state predicates, wait conditions, wake rules, and liveness failure modes.
- Go boundary: do not create new primitives in this lab. Use `syncx/`, channels, semaphores, queues, maps, clocks, and actor patterns as consumers.
- Comparison boundary: each puzzle should have at least two implementations when practical, then compare fairness, deadlock risk, starvation, cancellation, and backpressure.
- Verification boundary: an exercise is not complete until it has an invariant checker or stress case that can catch a known bad solution.
- Interview artifact: each exercise should produce a short explanation of the invariant, the failure mode, and why the chosen primitive fits.

## Implementation Checklist

- [ ] PrintInOrder
  - Core Concept: Force three functions to run in a fixed order using latches, channels, or semaphores.
  - Pros: Small warm-up that validates basic signaling.
  - Cons: Too small to expose fairness or backpressure.
  - Scenarios: LeetCode concurrency warm-up and latch practice.

- [ ] DiningPhilosophers
  - Core Concept: Multiple actors need two neighboring resources and can deadlock if everyone grabs one first.
  - Pros: Classic demonstration of Coffman conditions and lock ordering.
  - Cons: Toy problem unless multiple solutions are compared.
  - Scenarios: Resource hierarchy, asymmetric locking, Chandy-Misra, arbitrator designs.

- [ ] ReadersWriters
  - Core Concept: Readers can share access while writers need exclusivity, with fairness policy determining starvation behavior.
  - Pros: Directly maps to RWMutex design.
  - Cons: Reader-preference and writer-preference each starve somebody under the wrong workload.
  - Scenarios: RWMutex variants, database latches, shared caches.

- [ ] SleepingBarber
  - Core Concept: Customers wait in bounded chairs and a barber sleeps when no customers exist.
  - Pros: Condvar predicate-loop training.
  - Cons: Lost wakeup races appear if sleep and state checks are not atomic.
  - Scenarios: Multi-waiter coordination and monitor object practice.

- [ ] ProducerConsumerBoundedBuffer
  - Core Concept: Producers block when a buffer is full and consumers block when empty.
  - Pros: Canonical mutex+condvar/semaphore exercise.
  - Cons: Shutdown and cancellation add real complexity.
  - Scenarios: Blocking queue, backpressure, interview fundamentals.

- [ ] RendezvousAndMultiplex
  - Core Concept: Two or more goroutines must meet at a point before either proceeds, sometimes with role constraints.
  - Pros: Small exercise that clarifies semaphore, latch, channel, and barrier differences.
  - Cons: Too simple unless fairness and cancellation variants are added.
  - Scenarios: Little Book of Semaphores basics, start gates, pair handoff.

- [ ] CigaretteSmokers
  - Core Concept: A coordinator places two resources and exactly the smoker with the third resource should proceed.
  - Pros: Shows limitations of naive semaphore composition.
  - Cons: Requires pattern matching or extra coordination state.
  - Scenarios: Conditional wakeups and coordination beyond simple counting.

- [ ] H2OBuilder
  - Core Concept: Release groups containing exactly two hydrogen workers and one oxygen worker.
  - Pros: Teaches group rendezvous with roles.
  - Cons: Cancellation/error rollback makes the simple version much harder.
  - Scenarios: Barrier-like multi-role coordination and LeetCode 1117.

- [ ] RollerCoaster
  - Core Concept: A car departs only when a full group of riders is seated, then resets for the next group.
  - Pros: Practical reusable-barrier exercise.
  - Cons: Edge cases around shutdown and partial groups.
  - Scenarios: Batch admission and phase barriers.

- [ ] SantaClaus
  - Core Concept: Santa wakes for either all reindeer or a group of elves, with priority rules.
  - Pros: Exercises multiple wait queues and priority.
  - Cons: Complex enough to hide bugs without good tests.
  - Scenarios: Priority coordination and composite primitives.

- [ ] BarrierPuzzles
  - Core Concept: Groups of workers must repeatedly synchronize across phases.
  - Pros: Practices cyclic barriers, phasers, and broken-barrier behavior.
  - Cons: Reset and partial-failure rules make naive solutions fragile.
  - Scenarios: Roller coaster variants, reusable phase gates, simulation ticks.

- [ ] RiverCrossing
  - Core Concept: Form valid boat groups under role and safety constraints.
  - Pros: Forces explicit invariants and group admission.
  - Cons: More state-machine heavy than simple synchronization.
  - Scenarios: Missionaries/cannibals, hackers/serfs, bridge-crossing variants.

- [ ] UnisexBathroom
  - Core Concept: Allow bounded same-group occupancy while excluding the opposite group.
  - Pros: Combines capacity limits with fairness.
  - Cons: Starvation prevention is the real challenge.
  - Scenarios: Group mutual exclusion and policy comparison.

- [ ] MultithreadedWebCrawler
  - Core Concept: Traverse URLs concurrently while preserving a visited set and limiting concurrency.
  - Pros: Combines map, queue, rate limit, and cancellation.
  - Cons: Real networking adds nondeterminism; test with a fake graph.
  - Scenarios: Parallel BFS and practical service crawling.

- [ ] ParallelFizzBuzzAndFooBar
  - Core Concept: Coordinate multiple goroutines that print or emit tokens under modular predicates.
  - Pros: Quick practice for condition variables and semaphores.
  - Cons: Output-order puzzles do not teach resource contention deeply.
  - Scenarios: LeetCode concurrency set, predicate signaling warm-ups.

- [ ] StarvationAndLivelockLabs
  - Core Concept: Implement algorithms that are safe but fail to make progress under unfair scheduling or polite retry loops.
  - Pros: Separates safety from liveness in a tangible way.
  - Cons: Reproducing liveness failures may require scheduler hooks or stress loops.
  - Scenarios: RWMutex starvation, dining philosopher variants, backoff tuning.

- [ ] DiningSavages
  - Core Concept: Savages eat from a shared pot and the cook refills it only when empty, coordinated by counting and condition signaling.
  - Pros: Classic single-producer-on-empty rendezvous distinct from bounded-buffer patterns.
  - Cons: The "last one wakes the cook" handoff is easy to get wrong.
  - Scenarios: Little Book of Semaphores practice, refill-on-empty coordination.

- [ ] SushiBar
  - Core Concept: Bounded seats where, once full, arriving customers wait for the whole current party to leave before any sit (group constraint).
  - Pros: Combines capacity limits with a group-departure rule that breaks naive counting.
  - Cons: The transition between filling and draining states hides subtle races.
  - Scenarios: Group-admission puzzles, capacity-with-phase coordination.

- [ ] FaneuilHallSenateBus
  - Core Concept: Passengers board while a bus is present and the bus departs only after boarding completes, with arrivals-during-boarding handled correctly.
  - Pros: Exercises arrival-versus-departure races and barrier-like group release.
  - Cons: The boarding/departing handoff and late arrivals are the tricky cases.
  - Scenarios: Batch-departure coordination, Little Book of Semaphores practice.

- [ ] SearchInsertDelete
  - Core Concept: Three access classes where searchers share, inserters exclude other inserters, and deleters exclude everyone (Downey).
  - Pros: Generalizes readers-writers to a richer access lattice.
  - Cons: Getting the three exclusion relations right without starvation is delicate.
  - Scenarios: Fine-grained list access, RW-lock generalization, concurrent-collection rules.

- [ ] ZeroEvenOdd
  - Core Concept: Three goroutines print zero, odd, zero, even in strict order using handoff signaling (LeetCode 1116).
  - Pros: Compact three-way semaphore/handoff exercise with a precise output contract.
  - Cons: Too small to expose contention; purely an ordering drill.
  - Scenarios: LeetCode concurrency set, semaphore-handoff warm-up.

- [ ] TrafficLightController
  - Core Concept: Concurrent cars on two roads cross through a single mutable light, requiring mutual exclusion on the shared intersection (LeetCode 1279).
  - Pros: Maps a real-world mutual-exclusion scenario to a single-lock invariant.
  - Cons: Naive solutions over-synchronize and serialize same-direction traffic.
  - Scenarios: LeetCode concurrency, single-resource mutual exclusion practice.

- [ ] MorrisNoStarveMutex
  - Core Concept: Morris's three-semaphore algorithm builds a starvation-free mutex from weak semaphores using two gates.
  - Pros: Proves bounded waiting is achievable from weak primitives.
  - Cons: The two-gate turnstile structure is easy to misorder.
  - Scenarios: Fairness construction, weak-to-strong primitive building, starvation-freedom study.
