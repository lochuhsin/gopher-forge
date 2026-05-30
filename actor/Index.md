# actor Index

> Learning goal: model concurrency as isolated state machines that communicate by messages rather than shared memory.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: actor state is private; all external interaction goes through messages.
- Mailboxes are naturally MPSC: many senders, one actor receiver. Backpressure policy is part of the actor contract.
- Supervision turns failure into explicit policy: restart, stop, or escalate.
- Recommended build order from the merged TODO: mailbox on `queue/LockFreeMPSC`, actor plus scheduler, ask pattern with `syncx.Future`, behavior switching, one-for-one supervisor, then selective receive.
- Dependencies: consumes `queue/`, `scope/`, and `syncx.Future`; overlaps conceptually with `_lab/pattern` Active Object.
- Career signal: strongest for AI infra actor systems and event-driven services, less central for low-level HFT.
- Scope rule: focus on transferable actor concepts: mailbox, ownership, supervision, routing, backpressure, lifecycle, and request/reply.

## Reference Trail and Go Boundary

- Classic actor line: Hewitt actor model (`https://arxiv.org/abs/1008.1459`), Erlang/OTP supervision (`https://www.erlang.org/doc/system/sup_princ.html`), and Akka mailbox/dispatcher docs (`https://doc.akka.io/libraries/akka-core/current/typed/mailboxes.html`).
- Mental model: an actor is a private state machine plus mailbox plus lifecycle policy. The mailbox is the synchronization boundary.
- Go boundary: Go cannot kill goroutines or roll back side effects. Supervision must be cooperative: stop intake, cancel children, drain or reject mailbox, then restart explicitly.
- Mailbox boundary: FIFO, priority, bounded, lossy, selective receive, and dead-letter queues are different contracts; do not hide them behind one vague `Send`.
- Ask boundary: request/reply must handle timeout, late reply, actor death, and reply-handle revocation.
- Interview artifact: every actor item should name mailbox capacity policy, delivery guarantee, failure policy, and ownership of actor-local state.

## Implementation Checklist

- [ ] Mailbox
  - Core Concept: Each actor receives messages through an inbox, typically MPSC because many senders target one actor.
  - Pros: Isolates actor state and maps naturally to `queue/LockFreeMPSC`.
  - Cons: Hot actors create mailbox contention and require backpressure.
  - Scenarios: Actor runtime core, request routing, isolated workers.

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

- [ ] IsolatedProcess
  - Core Concept: A lightweight concurrent entity owns private state, communicates only by messages, and can fail independently.
  - Pros: Combines state isolation with failure containment.
  - Cons: Message copying, mailbox growth, and supervision policy become central design concerns.
  - Scenarios: Massive actor counts, workflow engines, symbol-sharded services.

- [ ] BehaviorSwitching
  - Core Concept: An actor can replace its receive function as protocol state changes.
  - Pros: Encodes state machines cleanly without exposing mutable state.
  - Cons: Hidden state transitions can make debugging harder.
  - Scenarios: Handshake protocols, workflow stages, protocol-state behaviors.

- [ ] StateMachineActor
  - Core Concept: Actor behavior is modeled as explicit states, events, actions, and state transitions.
  - Pros: Makes protocol correctness easier to test than a large unstructured receive function.
  - Cons: State explosion and timeout handling can make the model verbose.
  - Scenarios: Connection handshakes, workflow engines, order lifecycle state machines.

- [ ] ActorRef
  - Core Concept: A reference identifies an actor and exposes send/ask operations without exposing state.
  - Pros: Enables location transparency and API isolation.
  - Cons: Lifecycle and stale refs need explicit handling.
  - Scenarios: Local actor systems and future distributed actors.

- [ ] ActorRegistry
  - Core Concept: A registry maps stable names or keys to live actor references.
  - Pros: Decouples senders from actor construction and supports dynamic lookup.
  - Cons: Stale references, replacement races, and registry contention require policy.
  - Scenarios: Service discovery, partition ownership, supervisor-managed children.

- [ ] Scheduler
  - Core Concept: Actors with pending mailbox work are dispatched onto worker goroutines.
  - Pros: M:N scheduling avoids one goroutine per actor under idle workloads.
  - Cons: Fairness and starvation prevention are hard.
  - Scenarios: High-cardinality actors, runtime scheduling study.

- [ ] AskPattern
  - Core Concept: Request/reply sends a message containing a promise or reply channel.
  - Pros: Makes actor calls composable with futures.
  - Cons: Timeouts and actor death must complete the pending reply.
  - Scenarios: RPC-like actor methods and remote worker calls.

- [ ] RequestReplyCorrelation
  - Core Concept: Every request carries a correlation ID or reply handle so late or duplicate replies can be matched, ignored, or cancelled.
  - Pros: Prevents stale replies from corrupting later requests.
  - Cons: Requires cleanup for expired correlations and careful timeout handling.
  - Scenarios: Actor ask, RPC multiplexing, process-alias style reply cancellation.

- [ ] ReplyHandleRevocation
  - Core Concept: A caller can invalidate a reply handle after timeout so a late response cannot be delivered to the wrong waiter.
  - Pros: Fixes the common late-reply race in request/reply messaging.
  - Cons: Requires the receiver to observe failed delivery and define cleanup behavior.
  - Scenarios: Timed actor asks, RPC cancellation, fan-out first-success races.

- [ ] Supervisor
  - Core Concept: Parent actors observe child failures and decide restart, stop, or escalate.
  - Pros: Encodes failure policy explicitly.
  - Cons: Restart semantics can duplicate side effects if messages are not idempotent.
  - Scenarios: Fault-tolerant actor services and restartable workers.

- [ ] SupervisionTree
  - Core Concept: Supervisors form a hierarchy with restart strategies such as one-for-one, one-for-all, and escalate.
  - Pros: Makes failure containment and ownership visible.
  - Cons: Restart ordering and side-effect recovery are subtle.
  - Scenarios: Actor runtimes, long-running services, fault-isolation labs.

- [ ] FailureLink
  - Core Concept: Two actors are linked so failure in one propagates as a failure signal to the other.
  - Pros: Makes dependent lifetimes explicit and supports fail-fast groups.
  - Cons: Uncontrolled propagation can cascade failures too widely.
  - Scenarios: Parent-child lifecycle coupling, paired workers, crash-only subsystems.

- [ ] DeathWatchMonitor
  - Core Concept: One actor observes another and receives a termination notification without being killed by that failure.
  - Pros: Separates failure observation from failure propagation.
  - Cons: Monitors must handle races where the target exits before registration completes.
  - Scenarios: Supervisor observation, ask timeout cleanup, resource ownership tracking.

- [ ] RestartStrategy
  - Core Concept: A supervisor defines whether to restart only the failed child, all children, or the failed child plus later dependents.
  - Pros: Encodes dependency topology directly in failure recovery.
  - Cons: Wrong strategy can lose healthy work or leave dependent state inconsistent.
  - Scenarios: One-for-one workers, pipeline stage groups, dependent actor chains.

- [ ] RestartIntensityWindow
  - Core Concept: A supervisor stops restarting after too many failures within a time window.
  - Pros: Prevents endless crash loops from consuming resources.
  - Cons: Thresholds must distinguish transient faults from permanent bugs.
  - Scenarios: Fault containment, operational safeguards, resilience testing.

- [ ] ChildRestartPolicy
  - Core Concept: Each child declares whether it should always restart, restart only on failure, or never restart.
  - Pros: Separates worker lifecycle intent from supervisor strategy.
  - Cons: Misclassified children can either disappear silently or restart forever.
  - Scenarios: Permanent services, temporary jobs, transient background workers.

- [ ] ShutdownStrategy
  - Core Concept: A supervisor gives children a graceful stop deadline before forcing termination.
  - Pros: Bounds shutdown time while still allowing cleanup.
  - Cons: Forced termination can interrupt side effects and leave external resources inconsistent.
  - Scenarios: Rolling restarts, service shutdown, actor tree cleanup.

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
  - Pros: Models protocol handling where urgent or matching messages can bypass unrelated backlog.
  - Cons: Can starve skipped messages and complicates mailbox data structures.
  - Scenarios: Control messages, protocol state machines.

- [ ] SelectiveReceiveSaveQueue
  - Core Concept: Non-matching messages are preserved while the actor searches for a message matching the current receive pattern.
  - Pros: Makes selective receive semantics precise and testable.
  - Cons: Search is O(mailbox backlog) and skipped messages can accumulate.
  - Scenarios: Protocol waits, priority control messages, mailbox starvation labs.

- [ ] CallCastInfoLoop
  - Core Concept: A server actor distinguishes synchronous calls, asynchronous casts, and untyped informational messages.
  - Pros: Gives actor APIs a disciplined request/reply and fire-and-forget shape.
  - Cons: Call timeouts, backpressure, and unexpected messages still need policy.
  - Scenarios: Service actors, resource managers, request routers.

- [ ] ExitSignalHandling
  - Core Concept: Failure signals can either terminate an actor or be converted into ordinary messages for explicit handling.
  - Pros: Supports both fail-fast and recovery-oriented designs.
  - Cons: Converting failures to messages can hide serious invariant violations.
  - Scenarios: Supervisor loops, graceful degradation, linked actor groups.

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

- [ ] VirtualActor
  - Core Concept: Virtual actors (Microsoft Orleans grains) are always-addressable identities activated on demand and deactivated when idle, with single-activation guarantees.
  - Pros: Removes lifecycle management and gives location transparency at massive actor counts.
  - Cons: Single-activation and placement need a distributed directory, and turn-based reentrancy rules are subtle.
  - Scenarios: AI-agent runtimes, massive entity systems, cloud actor services.

- [ ] EventSourcedActor
  - Core Concept: An event-sourced actor (Akka Persistence) persists state-changing events plus periodic snapshots and replays them to recover on restart.
  - Pros: Durable, auditable state and clean recovery without saving mutable state directly.
  - Cons: Event-schema evolution and snapshot cadence add design burden, and replay can be slow.
  - Scenarios: Workflow engines, financial ledgers, durable stateful actors.

- [ ] StashUnstash
  - Core Concept: An actor stashes messages it cannot handle in the current behavior and unstashes them after a state change.
  - Pros: Cleanly handles out-of-order protocol messages without dropping them.
  - Cons: An unbounded stash can grow, and unstash ordering must be defined.
  - Scenarios: Handshake/init states, protocol actors, selective-receive alternative.

- [ ] ReceiveTimeout
  - Core Concept: Deliver a timeout message to an actor after its mailbox stays idle for a configured period.
  - Pros: Enables passivation and idle cleanup without an external timer per actor.
  - Cons: Timeout resets race with incoming messages and must be defined precisely.
  - Scenarios: Idle actor passivation, session expiry, virtual-actor deactivation.

- [ ] ClusterSharding
  - Core Concept: Distribute entity actors across nodes by entity ID with a shard coordinator that rebalances shards on membership change.
  - Pros: Scales stateful actors horizontally with location-transparent addressing.
  - Cons: Rebalancing, handoff, and single-owner guarantees require distributed coordination.
  - Scenarios: Sharded entity services, geo-distributed actors, large stateful systems.

- [ ] ConsistentHashRouter
  - Core Concept: Route messages to a pool member by consistent hash of a message key so same-key messages keep affinity across membership changes.
  - Pros: Stable key-to-worker affinity with minimal reshuffling when the pool resizes.
  - Cons: Hash skew can imbalance load, and affinity breaks during rebalancing windows.
  - Scenarios: Stateful worker pools, per-key ordering, partitioned actor routing.
