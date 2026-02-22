# Contributing to Quorapay

## Branch Strategy

Use the following branch names:

- `main` (production-level, finalized, submission-ready)
- `dev` (active development and integration branch)
- `feature/fault-tolerance`
- `feature/replication-consistency`
- `feature/time-sync-ordering`
- `feature/consensus-leader-election`

Optional integration branch:

- `integration/end-to-end-demo`

## Pull Request Rules

- No direct commits to `main`.
- No direct commits to `dev`; merge via pull request review.
- Each member works in their assigned feature branch and opens pull requests.
- Keep pull requests small, focused, and scoped to one concern.
- Link documentation updates to implementation changes in the same PR.
- Merge flow: `feature/*` -> `dev` during implementation, then `dev` -> `main` when production-ready.

## Commit Message Style

Use conventional prefixes and concise summaries.

Examples:

- `feat(replication): define quorum commit state transition notes`
- `fix(coordination): correct leader handoff sequence in design doc`
- `docs(architecture): add node role and failover sequence diagrams`

## Documentation Rules

- Shared documents belong under `docs/` (`00-overview` through `04-evaluation`).
- Per-member working notes belong only under:
- `docs/team/member1-fault-tolerance/`
- `docs/team/member2-replication/`
- `docs/team/member3-time-sync/`
- `docs/team/member4-consensus/`
- Do not overwrite other members' files.
- Each member creates and edits files only inside their own `docs/team/memberX-*` folder.
- Cross-cutting proposals are moved from member folders to shared `docs/` via PR review.

## Architecture Boundary

- Apache ZooKeeper is used for coordination only (leader election, membership, failover metadata).
- Transaction ledgers are stored on each node and replicated across nodes by Quorapay services.

## Before Merge Checklist

- Documentation for the change is updated.
- A minimal local smoke check is described in `docs/03-testing/` once test procedures are implemented.
- Branch is rebased or merged with latest `main` and conflicts are resolved.
