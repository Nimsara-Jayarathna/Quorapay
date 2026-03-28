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
- Member 4 related checks:
  - `./scripts/tests/test-election-consensus.sh`
  - `./scripts/tests/test-zk-outage-recovery.sh`
  - `./scripts/tests/test-fault-state-sequence.sh <port>`
  - `./scripts/tests/test-temporary-node-isolation-convergence.sh [isolation_seconds]`
  - `./scripts/tests/test-high-rate-throughput.sh [total_requests] [concurrency] [leader|round_robin]`

## Not in Scope
- Core application business logic.
- Long-running production automation.
- Secrets or environment credentials.

## Ownership
Owner: shared | Branch: shared
