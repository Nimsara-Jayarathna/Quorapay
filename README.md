# Quorapay

Quorapay is a fault-tolerant distributed payment system that ensures reliable and consistent transaction recording across multiple nodes. Using leader-based replication coordinated by Apache ZooKeeper, it provides quorum commits, automatic failover, and a replicated ledger accessible from any server despite failures or network delays.

## Goals

- Fault Tolerance: Keep payment processing available under node crashes, restarts, and partial network disruption.
- Replication & Consistency: Replicate transaction records across nodes with quorum-based commit guarantees.
- Time Sync: Enforce deterministic ordering rules with explicit handling of clock skew and delayed messages.
- Consensus: Use ZooKeeper-coordinated leader election and failover for primary node responsibility.
- Integration: Deliver a reproducible end-to-end prototype demo with observability and failure scenarios.

## High-Level Architecture

- Quorapay Node: Single node service implementation run as multiple instances via different configs.
- ZooKeeper Ensemble: Coordination layer for leader election, membership state, and failover signaling.
- Replicated Ledger: Per-node durable ledger replicas kept consistent through leader-based replication and quorum acknowledgements.

## Repository Layout

- `client/`: Client-facing interfaces (`cli/`, `web/`) for driving and observing prototype behavior.
- `cmd/quorapay-node/`: Node service entrypoint location.
- `configs/`: Node instance and ZooKeeper environment configuration.
- `deployments/`: Local and Docker orchestration assets for demos.
- `docs/`: Architecture, protocol, testing, evaluation, and team-specific working documents.
- `internal/`: Core domain modules (`api`, `coordination`, `replication`, `storage`, `timesync`, `consensus`).
- `scripts/`: Developer automation scripts.
- `test/`: Smoke and failure-focused test suites.
- `tools/`: Auxiliary tooling used by development and evaluation workflows.

## Typical Demo Flow

1. Start ZooKeeper.
2. Start three Quorapay node instances with different node configs.
3. Submit payment requests from CLI or web client.
4. Observe replication and quorum commit behavior.
5. Terminate the leader node process to trigger failover.
6. Continue submitting requests after leadership recovery.
7. Verify each node ledger converges to the same committed state.

## Planned Interfaces

- Client Endpoints: Payment submission, payment status query, and ledger read/query interfaces for operators and demo clients.
- Internal Replication Endpoints: Leader-to-follower append, acknowledgement, commit propagation, heartbeat, and catch-up synchronization interfaces.

## Collaboration Workflow

- Work is split by dedicated feature branches aligned to core system concerns.
- `dev` is the shared integration branch used during active development.
- `main` is the production-level, finalized branch and stays stable.
- Merge flow is `feature/*` -> `dev` -> `main`, with pull requests at each stage.
- No direct commits to `main`.
- Shared architecture and protocol decisions are documented under `docs/` and reviewed in PRs.

## Web Client (React)

Run the demo web client from `client/web`:

```bash
cd client/web
cp .env.example .env
npm install
npm run dev
```

Build command:

```bash
npm run build
```
