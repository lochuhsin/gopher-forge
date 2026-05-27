# actor Index

> Learning goal: model concurrency as isolated state machines that communicate by messages rather than shared memory.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: actor state is private; all external interaction goes through messages.
- Mailboxes are naturally MPSC: many senders, one actor receiver. Backpressure policy is part of the actor contract.
- Supervision turns failure into explicit policy: restart, stop, or escalate.
- Recommended build order from the merged TODO: mailbox on `queue/LockFreeMPSC`, actor plus scheduler, ask pattern with `syncx.Future`, behavior switching, one-for-one supervisor, then selective receive.
- Dependencies: consumes `queue/`, `scope/`, and `syncx.Future`; overlaps conceptually with `_lab/pattern` Active Object.
- Career signal: strongest for AI infra/Ray-style systems and event-driven services, less central for low-level HFT.
- Scope rule: focus on transferable actor concepts: mailbox, ownership, supervision, routing, backpressure, lifecycle, and request/reply.

## Implementation Checklist

- [ ] Mailbox
  - Core Concept: Each actor receives messages through an inbox, typically MPSC because many senders target one actor.
  - Pros: Isolates actor state and maps naturally to `queue/LockFreeMPSC`.
  - Cons: Hot actors create mailbox contention and require backpressure.
  - Scenarios: Actor runtime core, request routing, Ray-like workers.

- [ ] PriorityMailbox
  - Core Concept: The mailbox orders messages by priority or control/data class instead of pure FIFO.
  - Pros: Lets shutdown, health, and control messages bypass normal backlog.
  - Cons: Can starve low-priority messages and complicates fairness proofs.
  - Scenarios: Supervisor signals, overload control, protocol actors.

- [ ] ActorInterface
  - Core Concept: An actor owns state and handles one message at a time through a receive function.
  - Pros: Eliminates internal data races by construction.
  - Cons: Long handlers block that actor's mailbox.
  - Scenarios: Game entities, exchange symbols, workflow objects.

- [ ] BehaviorSwitching
  - Core Concept: An actor can replace its receive function as protocol state changes.
  - Pros: Encodes state machines cleanly without exposing mutable state.
  - Cons: Hidden state transitions can make debugging harder.
  - Scenarios: Handshake protocols, workflow stages, Erlang-style behaviors.

- [ ] ActorRef
  - Core Concept: A reference identifies an actor and exposes send/ask operations without exposing state.
  - Pros: Enables location transparency and API isolation.
  - Cons: Lifecycle and stale refs need explicit handling.
  - Scenarios: Local actor systems and future distributed actors.

- [ ] Scheduler
  - Core Concept: Actors with pending mailbox work are dispatched onto worker goroutines.
  - Pros: M:N scheduling avoids one goroutine per actor under idle workloads.
  - Cons: Fairness and starvation prevention are hard.
  - Scenarios: High-cardinality actors, runtime scheduling study.

- [ ] AskPattern
  - Core Concept: Request/reply sends a message containing a promise or reply channel.
  - Pros: Makes actor calls composable with futures.
  - Cons: Timeouts and actor death must complete the pending reply.
  - Scenarios: RPC-like actor methods and Ray remote calls.

- [ ] Supervisor
  - Core Concept: Parent actors observe child failures and decide restart, stop, or escalate.
  - Pros: Encodes failure policy explicitly.
  - Cons: Restart semantics can duplicate side effects if messages are not idempotent.
  - Scenarios: Erlang/OTP-style resilience and service supervision.

- [ ] SupervisionTree
  - Core Concept: Supervisors form a hierarchy with restart strategies such as one-for-one, one-for-all, and escalate.
  - Pros: Makes failure containment and ownership visible.
  - Cons: Restart ordering and side-effect recovery are subtle.
  - Scenarios: Actor runtimes, long-running services, fault-isolation labs.

- [ ] BoundedMailbox
  - Core Concept: Mailboxes have capacity and define overflow behavior: block, drop, replace, or dead-letter.
  - Pros: Provides backpressure and protects memory.
  - Cons: Every overflow policy has semantic tradeoffs.
  - Scenarios: Slow consumer handling and actor overload control.

- [ ] DeadLetterQueue
  - Core Concept: Messages that cannot be delivered are routed to a diagnostic sink.
  - Pros: Makes actor death, full mailboxes, and stale refs observable.
  - Cons: Dead-letter handling can itself become a sink or leak if unbounded.
  - Scenarios: Debugging, ask timeout cleanup, supervisor reporting.

- [ ] Router
  - Core Concept: A router distributes messages among a pool of actors by hash, round-robin, or load.
  - Pros: Scales stateless or partitioned actors.
  - Cons: Ordering and affinity can be broken if routing policy is wrong.
  - Scenarios: Symbol sharding, request fan-out, worker pools.

- [ ] SelectiveReceive
  - Core Concept: An actor can scan or filter mailbox messages by pattern instead of strict FIFO.
  - Pros: Models Erlang-style protocol handling.
  - Cons: Can starve skipped messages and complicates mailbox data structures.
  - Scenarios: Control messages, protocol state machines.

- [ ] GracefulShutdown
  - Core Concept: Stop accepting new messages, drain or reject queued work, and notify dependents.
  - Pros: Prevents leaks and dangling asks.
  - Cons: Shutdown ordering across actor graphs is complex.
  - Scenarios: Service shutdown, supervisor restarts, rolling deploys.

- [ ] ActorLifecycleHooks
  - Core Concept: Actors expose start, stop, pre-restart, and post-restart hooks around mailbox processing.
  - Pros: Gives resource management a structured place.
  - Cons: Hooks can introduce side effects that complicate restart safety.
  - Scenarios: Opening connections, cleanup, supervision tests.
