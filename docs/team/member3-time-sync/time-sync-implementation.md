# Member 3 Time Synchronization - Implementation

## Objective Coverage
Implemented timestamp quality and ordering support for distributed event correlation and debugging across nodes.

## What Was Implemented

1. Logical ordering for distributed events
- Lamport clock is enforced on:
  - leader write path (`POST /pay`) via `Send()`
  - follower append path (`POST /internal/append`) via `Receive()`
- Deterministic ordering test added for close timestamps:
  - logical timestamp first
  - `node_id` tie-break

2. Persistent timestamp fields
- `physical_time` and `logical_time` are persisted per payment record.
- Ledger API and UI now expose `logical_time` for inspection.

3. Reorder policy in ledger view path
- Payment list ordering now prioritizes:
  - `logical_time` ASC
  - `log_index` ASC
  - `id` ASC

4. Clock skew validation policy
- Follower append path validates physical timestamps and clock skew.
- Policy includes:
  - warning threshold (`SKEW_WARN_MS`)
  - reject threshold (`SKEW_REJECT_MS`)
  - old message reject threshold (`MAX_MESSAGE_AGE_MS`)
  - future drift reject threshold (`MAX_FUTURE_DRIFT_MS`)
- Thresholds are configurable through environment variables.

5. Time-sync health visibility
- `/status` now includes:
  - `lamport_time`
  - `clock_skew_ms`
- Web status panel displays both fields.

## Configurable Environment Variables
- `SKEW_WARN_MS` (default `300`)
- `SKEW_REJECT_MS` (default `500`)
- `MAX_MESSAGE_AGE_MS` (default `2000`)
- `MAX_FUTURE_DRIFT_MS` (default `500`)

## Validation Summary
- Unit tests added/updated across `internal/timesync`, `internal/api`, and `internal/storage`.
- Manual tests confirmed:
  - old/future timestamp rejection on `/internal/append`
  - Lamport progression on append/write paths
  - status exposes live time-sync health fields.

## Trade-off Notes
- Strict skew rejection improves timestamp trust but may reject valid traffic under unstable clocks.
- Warning threshold helps observe drift before hard rejects occur.
- Logical ordering (Lamport) ensures deterministic sequence even with delayed or near-simultaneous events.
