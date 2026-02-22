# Quorapay Implementation Plan

## Objectives

- Build a prototype distributed payment system with leader-based replication and fault tolerance.
- Maintain consistent replicated ledgers across multiple node instances.
- Provide automatic leader failover coordinated through Apache ZooKeeper.
- Demonstrate deterministic transaction ordering with explicit time-sync handling.
- Deliver a clear integration demo and evaluation artifacts.

## Core Decisions

- Single monorepo for all components and documentation.
- One node service codebase deployed as multiple instances through per-node configuration.
- Apache ZooKeeper is the coordination mechanism; no heavy external consensus framework.
- Quorum commit is required before transactions become visible as committed.
- Documentation-first collaboration to align protocol and failure semantics before full implementation.

## Milestones

### M1: Repository Baseline and Team Conventions

- Establish monorepo folder structure.
- Publish contribution workflow and branch strategy.
- Define shared vs member-isolated documentation areas.

### M2: Interfaces and Protocol Specifications

- Define client-facing API contract and error model.
- Define internal replication RPC/message contracts.
- Document leader, follower, and recovery state transitions.

### M3: Coordination and Leader Election Design

- Specify ZooKeeper znode layout and ephemeral node usage.
- Define leader election flow, fencing strategy, and failover triggers.
- Define node startup and rejoin behavior.

### M4: Replication and Ledger Consistency Design

- Specify append, ack, quorum commit, and commit index progression.
- Define ledger data model, idempotency rules, and replay/catch-up behavior.
- Define durability boundaries for local per-node storage.

### M5: Time Sync, Failure Testing, and Validation Plan

- Define timestamp policy and ordering guarantees under skew.
- Define failure test matrix (leader crash, follower lag, partition, delayed messages).
- Define smoke and fault-injection test procedures and expected outcomes.

### M6: Integration Demo and Evaluation Package

- Assemble end-to-end demo script and runbook.
- Validate failover and post-recovery ledger convergence.
- Produce evaluation summary for reliability, consistency, and recovery behavior.

## Member Area Mapping

- Member 1 (`feature/fault-tolerance`): Failure handling model, recovery procedures, and resilience test scenarios.
- Member 2 (`feature/replication-consistency`): Replication protocol, commit semantics, and ledger convergence guarantees.
- Member 3 (`feature/time-sync-ordering`): Time synchronization policy and transaction ordering rules.
- Member 4 (`feature/consensus-leader-election`): ZooKeeper coordination, leader election, and leadership transitions.

## Documentation Workflow Rules

- Shared canonical documents are maintained in `docs/00-overview` to `docs/04-evaluation`.
- Individual drafts and experiments stay in each member's `docs/team/memberX-*` folder.
- Members do not edit files owned by other member folders.
- Cross-team decisions are promoted from member notes into shared docs through reviewed PRs.
- Every protocol or behavior change must update the corresponding shared documentation before merge.

## Definition of Done (Prototype Level)

- Multi-node prototype runs with one leader and follower replicas under ZooKeeper coordination.
- Quorum commit behavior is demonstrated with consistent ledger state after normal operation.
- Leader failure triggers automatic failover and service recovery without ledger divergence.
- Failure scenarios in the agreed test matrix are executed and documented.
- Demo runbook and evaluation report are complete, reproducible, and reviewable.
