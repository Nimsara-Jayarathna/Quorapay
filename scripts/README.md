# scripts

## Overview
Helper scripts for developer workflows and reproducible demos. Scripts should improve local productivity without embedding core runtime logic.

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
- 3 nodes:
  - `CLUSTER_SIZE=3 ./scripts/run-zk.sh`
  - `CLUSTER_SIZE=3 ./scripts/run-nodes.sh`
- 5 nodes:
  - `CLUSTER_SIZE=5 ./scripts/run-zk.sh`
  - `CLUSTER_SIZE=5 ./scripts/run-nodes.sh`
- 7 nodes:
  - `CLUSTER_SIZE=7 ./scripts/run-zk.sh`
  - `CLUSTER_SIZE=7 ./scripts/run-nodes.sh`

Stop cluster:
- `CLUSTER_SIZE=<same-size> ./scripts/stop-nodes.sh`
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
