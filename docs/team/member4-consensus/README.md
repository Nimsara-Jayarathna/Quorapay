# member4-consensus

## Overview
Isolated notes and drafts for leader election and coordination semantics. Focus on ZooKeeper-based leadership transitions and safety boundaries.

## Contents
- Leader election flow proposals.
- Term and role transition notes.
- Coordination safety constraints.
- Failover trigger and fencing considerations.
- Local failover drill support for verifying re-election behavior.

## Not in Scope
- Replication data-path ownership from other tracks.
- UI or demo client concerns.
- Edits to other members' team folders.

## Ownership
Owner: member4-consensus | Branch: feature/consensus-leader-election

## Agreement Upgrade

This track now upgrades the original baseline election into a ZooKeeper-backed agreement layer with Raft-style leadership semantics:

- election znodes still decide which node is leader-eligible
- `/leader` is the single source of truth for actual leadership
- `/term` is a persistent monotonic fencing token
- `/log_head` defines the leader-issued log index contract for future replication work

The design intentionally demonstrates consensus principles without importing a full Raft or Paxos library. ZooKeeper is the coordination substrate for:

- membership liveness
- leader eligibility
- lease ownership
- term/epoch advancement
- stale-leader fencing

## Coordination Model

Current ZooKeeper layout under `ZK_ROOT`:

- `/quorapay/members/<NODE_ID>`: ephemeral membership, data=`BASE_URL`
- `/quorapay/election/candidate-*`: ephemeral sequential election candidates, data=`NODE_ID`
- `/quorapay/leader`: ephemeral leader lease, JSON payload with `leader_id`, `leader_url`, `term`, `since`, `lease_id`
- `/quorapay/term`: persistent integer string, monotonically increasing
- `/quorapay/log_head`: persistent integer string, latest leader-issued log index contract

Leadership flow:

1. Every node joins `/election` with an ephemeral sequential candidate.
2. The lowest candidate is leader-eligible, but not leader yet.
3. The leader-eligible node must acquire the ephemeral `/leader` lease.
4. After lease creation, it bumps `/term` using ZooKeeper version-based CAS.
5. It updates `/leader` with the new term.
6. All nodes derive `leader_id`, `leader_url`, and `term` from `/leader`, not from the election candidate ordering directly.

This prevents a stale process from continuing to claim leadership after it loses ZooKeeper session ownership.

## Safety Guarantees

- A node only reports `LEADER` if it still owns the `/leader` ephemeral znode in the current ZooKeeper session.
- Losing ZooKeeper connectivity or session validity immediately drops the local role to `UNKNOWN`.
- The `/term` value only advances on leadership change, which gives us a monotonic fencing token.
- A restarted former leader cannot safely act as leader again unless it becomes leader-eligible and re-acquires `/leader`.
- Followers do not advance `/log_head`; only the current lease holder may do so.

These are the agreement hooks Member 2 can build on later for append validation, term checking, and log index issuance.

## Status and Metrics

`GET /status` now exposes additive agreement fields for demo and evaluation:

- `term`
- `log_head`
- `election_count`
- `last_election_ms`
- `status_refresh_ms`

These make leadership changes visible and measurable during failure drills.

## Failure Drill Endpoint

For local testing we keep the node-local admin endpoint `POST /admin/shutdown`. This is still a deliberate test hook rather than a production control-plane feature.

Why this remains useful:

- It lets us terminate one chosen node without manual PID discovery.
- It exercises the normal shutdown path, so DB close and ZooKeeper session teardown are part of the test.
- It makes the lease-loss and re-election flow observable in a repeatable way.

## Failure Scenario Tests

Recommended validation steps for the assignment:

### 1. Initial leader establishment

1. Check ZooKeeper readiness: `./scripts/run-zk.sh`
2. Start nodes: `./scripts/run-nodes.sh`
3. Read:
   - `curl http://localhost:8001/status`
   - `curl http://localhost:8002/status`
   - `curl http://localhost:8003/status`
4. Verify:
   - exactly one node reports `LEADER`
   - `/leader` exists in ZooKeeper
   - `term >= 1`

### 2. Leader crash / forced failover

1. Kill the current leader: `./scripts/kill-leader.sh`
2. Re-read `/status` on the surviving nodes
3. Verify:
   - exactly one new leader exists
   - `term` increased by 1
   - the old leader is unreachable

### 3. Leader restart / stale leader fencing

1. After a leader crash test, restart the old leader process only: `./scripts/run-node.sh A` (or `B` / `C` as needed)
2. Re-read all `/status` responses
3. Verify:
   - the restarted old leader reports `FOLLOWER` unless it truly re-acquired the lease
   - the node reports the current `leader_id`, `leader_url`, and `term`
   - there is still exactly one leader

### 4. Temporary ZooKeeper outage

1. Stop or interrupt the local ZooKeeper service briefly
2. Observe node `/status`
3. Verify:
   - nodes drop to `UNKNOWN`
   - `zk_error` is populated
4. Restore ZooKeeper
5. Verify:
   - leadership is re-established
   - a fresh term is issued when a new lease is acquired after session loss

### 5. Targeted single-node shutdown

1. Call `curl -X POST http://localhost:8001/admin/shutdown`
2. Verify that node A becomes unreachable
3. Check the remaining nodes for correct leader continuity or re-election depending on which node was terminated

## Performance and Overhead Discussion

Why the ZooKeeper-backed lease reduces split-brain risk:

- Actual leadership is tied to a single ephemeral znode (`/leader`).
- When the leader loses session ownership, ZooKeeper removes the lease automatically.
- Other nodes then observe lease removal and can safely elect and fence in a new leader.

Why the term is a fencing token:

- Every new leadership acquisition increments `/term`.
- Any future write path can require the caller's term to match the current lease term.
- A previously crashed or partitioned leader with an old term is therefore stale and can be rejected.

Main overhead sources:

- reads on `/leader` and `/log_head`
- watches on `/election` and `/leader`
- CAS updates on `/term` when leadership changes

Optimizations used or intended:

- watches plus periodic reconcile instead of busy polling only
- term bumps only on leadership transition, not on every loop
- cached in-memory status for `/status` responses
- stable-loop refresh tracking via `status_refresh_ms` so reconcile cadence can be tuned if needed later

## Current Limitation

- We do not have a dedicated single-node restart controller yet.
- Single-node rejoin is supported with `./scripts/run-node.sh <A|B|C>`, but it still uses the fixed local node mapping.
- For a full reset of the demo cluster, stop any remaining local node processes and start the standard three-node set again with `./scripts/run-nodes.sh`.
