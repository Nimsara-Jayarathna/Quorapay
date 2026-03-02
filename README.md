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
- `deployments/`: Deployment and environment assets for demos.
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

## Baseline Run

This baseline milestone wires only ZooKeeper membership/leader election plus per-node SQLite initialization. It does not include payment submission, replication, quorum commit, or consensus libraries beyond ZooKeeper coordination.

1. Start ZooKeeper:

```bash
./scripts/run-zk.sh
```

`run-zk.sh` is now a readiness check only. It verifies that your locally installed ZooKeeper is already listening on the configured `ZK_ADDR` (default `localhost:2181`).

2. Start the three node instances (A/B/C) from the same codebase:

```bash
cp .env.example .env
./scripts/run-nodes.sh
```

The root `.env` file is used by `scripts/run-nodes.sh`. Set `CORS_ALLOWED_ORIGINS` there if you want the browser-based web client to call the API:

```bash
CORS_ALLOWED_ORIGINS=http://localhost:5173
ZK_ADDR=localhost:2181
ZK_ROOT=/quorapay
```

3. Verify status and leader election:

```bash
curl http://localhost:8001/status
curl http://localhost:8002/status
curl http://localhost:8003/status
```

One node should report `"role":"LEADER"` and the others should report `"role":"FOLLOWER"`.

4. Verify each node can read its local ledger (empty for the baseline):

```bash
curl http://localhost:8001/ledger
curl http://localhost:8002/ledger
curl http://localhost:8003/ledger
```

5. Trigger failover by killing the current leader, then re-check `/status`:

```bash
./scripts/kill-leader.sh
curl http://localhost:8001/status
curl http://localhost:8002/status
curl http://localhost:8003/status
```

You can also terminate a specific node directly through the node API or the web client:

```bash
curl -X POST http://localhost:8002/admin/shutdown
```

This local-only admin endpoint is intended for failover drills so you can verify that leader election updates correctly when an individual node exits.

6. Stop background node processes when finished:

```bash
./scripts/stop-nodes.sh
```

Runtime SQLite files are created under `./data/nodeA/ledger.db`, `./data/nodeB/ledger.db`, and `./data/nodeC/ledger.db`. The `data/` directory is intentionally ignored by git.

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
