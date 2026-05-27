# Package Index Documentation Design

## Goal

Create a package-level learning index for every implementation-oriented folder in this repository, excluding `docs/` and including `_lab/*`.

The repository's purpose is to learn concurrency from low-level synchronization primitives through higher-level patterns. Each `Index.md` should turn a folder into a focused curriculum: what belongs in the package, what is already implemented, what remains, and why each item matters.

## Scope

Generate or update `Index.md` in:

- `syncx/`
- `queue/`
- `stack/`
- `memory/`
- `hazard/`
- `reclamation/`
- `rcu/`
- `map/`
- `deque/`
- `park/`
- `scope/`
- `clock/`
- `crdt/`
- `parallel/`
- `ratelimit/`
- `actor/`
- `arena/`
- `_lab/pattern/`
- `_lab/verify/`
- `_lab/excercise/`

Do not generate `Index.md` for `docs/`.

## Status Rules

Use actual source files as the primary status source.

- `[x]` means a usable implementation exists.
- `[~]` means a scaffold, partial implementation, or non-production implementation exists.
- `[ ]` means the item is planned but not implemented.

Existing `README.md` and `TODO.md` files can provide roadmap context, but they must not override source-level evidence when status differs.

## File Structure

Each `Index.md` should use this shape:

```md
# <package> Index

> Learning goal: ...

## Implementation Checklist

- [x] ImplementationName
  - Core Concept: ...
  - Pros: ...
  - Cons: ...
  - Scenarios: ...
```

For larger packages such as `syncx/`, split the checklist into concept families such as locks, barriers, latches, semaphores, condition variables, futures, and STM. For smaller packages, keep a single checklist.

## Content Requirements

Every implementation item must include:

- Core Concept
- Pros
- Cons
- Scenarios

Items should be written as concise learning notes, not marketing copy. The audience is someone using this repository to build a deep concurrency mental model.

## Package Boundary Rules

- Keep low-level synchronization primitive families under `syncx/`.
- Keep data structures in their package folders: `queue/`, `stack/`, `deque/`, `map/`.
- Keep memory-ordering and reclamation topics separate: `memory/`, `hazard/`, `reclamation/`, `rcu/`.
- Keep higher-level coordination and distributed patterns separate: `scope/`, `parallel/`, `ratelimit/`, `actor/`, `clock/`, `crdt/`.
- Keep exercise, pattern, and verification material under `_lab/*`.

## Verification

After writing the indexes:

- Confirm all required folders have `Index.md`.
- Confirm no `docs/**/Index.md` was created.
- Run a quick markdown/content sanity check with shell tools.
- Do not run Go tests unless implementation code is changed.

