# scripts

## Overview
Helper scripts for developer workflows and reproducible demos. Scripts should improve local productivity without embedding core runtime logic.

## Environment Files
- Shared topology: `.env.common`
- Node runtime: `.env.node`
- Admin runtime: `.env.admin`
- Commented templates:
  - `.env.common.example`
  - `.env.node.example`
  - `.env.admin.example`

Default load behavior:
- Node scripts (`run-nodes.sh`, `run-node.sh`) load `.env.common` and `.env.node`.
- Admin script (`run-admin.sh`) loads `.env.common` then `.env.admin`.

## Contents
- Cluster startup and shutdown helpers.
- Failure simulation utilities for demo scenarios.
- Demo data generation scripts.
- Local convenience wrappers for repeated commands.

## Test Scripts
- Consensus and failover validation scripts are in `scripts/tests/`.
- Cluster size is configurable via `CLUSTER_SIZE` (odd integer >= 3).
- Base port is configurable via `CLUSTER_BASE_PORT` (default `8001`).
- Examples:
  - `CLUSTER_SIZE=3` -> `A:8001,B:8002,C:8003`
  - `CLUSTER_SIZE=5` -> `A:8001,B:8002,C:8003,D:8004,E:8005`
  - `CLUSTER_SIZE=7` -> `A:8001,B:8002,C:8003,D:8004,E:8005,F:8006,G:8007`
- Optional override: set `NODES` directly to any custom mapping.

## Startup Recipes
- Configure topology in `.env.common` (no CLI `CLUSTER_SIZE=...` required).
- 3 nodes:
  - set `CLUSTER_SIZE=3` in `.env.common`
  - `./scripts/run-zk.sh`
  - `./scripts/run-nodes.sh`
- 5 nodes:
  - set `CLUSTER_SIZE=5` in `.env.common`
  - `./scripts/run-zk.sh`
  - `./scripts/run-nodes.sh`
- 7 nodes:
  - set `CLUSTER_SIZE=7` in `.env.common`
  - `./scripts/run-zk.sh`
  - `./scripts/run-nodes.sh`

Stop cluster:
- `./scripts/stop-nodes.sh`

Admin control service:
- Start: `./scripts/run-admin.sh`
- Stop: `./scripts/stop-admin.sh`
- Default port: `8090` (override with `ADMIN_PORT`)
- Uses separate env file: `.env.admin` (override path via `ADMIN_ENV_FILE=/path/to/file ./scripts/run-admin.sh`)
- `run-admin.sh` loads `.env.common` first (cluster topology), then `.env.admin` (admin overrides)
- CORS origins for browser calls: `ADMIN_CORS_ALLOWED_ORIGINS` (default `http://localhost:5173`)
- APIs:
  - `POST http://localhost:8090/admin/node/{id}/start`
  - `POST http://localhost:8090/admin/node/{id}/stop`
  - `POST http://localhost:8090/admin/node/{id}/restart`
  - Optional auth: `Authorization: Bearer $ADMIN_API_TOKEN`

- Member 4 related checks:
  - `./scripts/tests/test-election-consensus.sh`
  - `./scripts/tests/test-zk-outage-recovery.sh`
  - `./scripts/tests/test-fault-state-sequence.sh <port>`
  - `./scripts/tests/test-temporary-node-isolation-convergence.sh [isolation_seconds]`
  - `./scripts/tests/test-high-rate-throughput.sh [total_requests] [concurrency] [leader|round_robin]`
  - `./scripts/tests/run-all-tests.sh` (single entrypoint for automated suite)

Run all automated tests:
- `./scripts/tests/run-all-tests.sh`

Run all tests with custom parameters:
- `CYCLES=5 LOAD_REQUESTS=300 LOAD_CONCURRENCY=20 LOAD_MODE=round_robin ./scripts/tests/run-all-tests.sh`

Include manual-interaction tests (`zk outage`, optional fault-state sequence):
- `INCLUDE_MANUAL=1 FAULT_STATE_PORT=8003 ./scripts/tests/run-all-tests.sh`

High-rate throughput tuning:
- `RETRY_ATTEMPTS` retry count for transient `503`/transport failures (default `2`)
- `RETRY_SLEEP_SECONDS` delay between retries (default `0.2`)
- `MAX_FAILURES` allowed failed requests before test fails (default `0`)
- Example stress profile:
  - `LOAD_REQUESTS=120 LOAD_CONCURRENCY=12 MAX_FAILURES=60 ./scripts/tests/test-high-rate-throughput.sh 120 12 leader`

## Not in Scope
- Core application business logic.
- Long-running production automation.
- Secrets or environment credentials.

## Ownership
Owner: shared | Branch: shared
