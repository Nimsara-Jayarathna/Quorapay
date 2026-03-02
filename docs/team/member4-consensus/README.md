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

## Baseline Failover Drill Endpoint

For the baseline milestone we added a local-only admin endpoint, `POST /admin/shutdown`, on each node. This is a deliberate testing aid for the leader-election track rather than a production control plane feature.

Why this was added:
- It allows the web client and simple `curl` commands to terminate one specific node without relying on terminal-side PID lookups.
- It gives a deterministic way to test that ZooKeeper removes the node's ephemeral membership and election znodes when the process exits.
- It lets us observe that the remaining nodes recompute leadership and that `/status` reflects the new leader after failover.
- It also lets us restart the stopped node and confirm it rejoins membership and re-enters the election set cleanly.

Why this is acceptable in the baseline:
- The current milestone is focused on connectivity, liveness, and leader-election correctness, not hardened admin security.
- A self-termination endpoint is the smallest implementation that exercises the real shutdown path of the node process.
- Because the node exits normally, database close and ZooKeeper session teardown are exercised as part of the test, which is more representative than only killing arbitrary PIDs from outside the process.

Expected validation flow:
1. Start ZooKeeper and all three nodes.
2. Confirm one node reports `LEADER` and two report `FOLLOWER`.
3. Call `POST /admin/shutdown` on a chosen node, especially the current leader.
4. Confirm the stopped node becomes unreachable.
5. Confirm one of the remaining followers becomes the new leader.
6. Restart the stopped node and confirm it rejoins as a member and receives an updated role.

Operational commands used for this validation:
1. Check local ZooKeeper readiness: `./scripts/run-zk.sh`
2. Start the baseline cluster: `./scripts/run-nodes.sh`
3. Kill only the elected leader: `./scripts/kill-leader.sh`
4. Stop all local nodes cleanly: `./scripts/stop-nodes.sh`
5. Terminate a chosen node directly: `curl -X POST http://localhost:8001/admin/shutdown`
6. Restore the baseline cluster after a kill/stop drill: `./scripts/run-nodes.sh`

Current limitation:
- We do not have a dedicated single-node restart controller yet.
- The practical restart path in this milestone is to stop any remaining local node processes and start the standard three-node set again with `./scripts/run-nodes.sh`.
