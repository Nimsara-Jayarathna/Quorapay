# Member 4 Final Documentation: Consensus and Agreement Algorithms

## Objective
Ensure distributed payment servers agree on log storage, indexing, and retrieval policies with safe leadership transitions and failure recovery.

## Implemented Scope
- ZooKeeper-backed agreement model with Raft-style leadership semantics.
- Single active leader lease (`/leader`) and monotonic fencing term (`/term`).
- Leader-issued log progress contract (`/log_head`).
- Leader election via ephemeral sequential candidates.
- Stale-leader fencing on restart/re-election.
- Automated validation scripts for failover and recovery behavior.

## Agreement Model
- Candidate ordering decides leader eligibility.
- Lease ownership decides actual active leader.
- Term increases on leadership transition (fencing token).
- Followers derive leader and term from lease state.

Safety properties:
- single-leader invariant
- monotonic term progression
- stale-leader rejection/fencing
- follower reconvergence after disruption

## Final 7-Node Validation Run

Run used:

```bash
CLUSTER_SIZE=7 ./scripts/stop-nodes.sh
rm -rf data/node* data/logs data/pids
CLUSTER_SIZE=7 ./scripts/run-zk.sh
CLUSTER_SIZE=7 ./scripts/run-nodes.sh
sleep 5
CLUSTER_SIZE=7 MAX_FAILURES=5 ./scripts/tests/run-all-tests.sh
```

Observed:
- Election Consensus: **PASS**
- Follower Rejoin Convergence: **PASS**
- Repeated Restart No Divergence: **PASS**
- Temporary Node Isolation Convergence: **PASS**
- High Rate Throughput: **FAIL** under strict threshold

Throughput test summary:
- total_requests=120
- concurrency=12
- target_mode=leader
- success=92
- failures=28
- throughput_rps=10
- latency_avg_ms=437
- latency_p95_ms=1092
- latency_max_ms=1349
- error breakdown: 503=27, 409=1

Interpretation:
- Consensus correctness and recovery behavior are validated by the first four passing scenarios.
- Remaining gap is burst-load throughput/availability under strict failure threshold, not agreement safety.

## Conclusion
Member 4 consensus/agreement objectives are implemented and validated for leader election, fencing, failover correctness, and post-failure convergence. The remaining limitation is high-burst load performance behavior (transient write-path rejects), which is a tuning/capacity concern.

## Embedded Evidence

![Member 4 Evidence 1](./assets/Screenshot%202026-03-29%20at%2000.29.52.png)

![Member 4 Evidence 2](./assets/Screenshot%202026-03-29%20at%2000.30.00.png)

![Member 4 Evidence 3](./assets/Screenshot%202026-03-29%20at%2000.30.07.png)

![Member 4 Evidence 4](./assets/Screenshot%202026-03-29%20at%2000.41.44.png)
