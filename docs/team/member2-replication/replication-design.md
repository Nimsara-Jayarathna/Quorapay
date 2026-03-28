# Replication & Consistency Design (Member 2)

## 1. Overview

This module is responsible for ensuring that payment transactions are reliably replicated across multiple nodes in a distributed environment.

The system uses a **leader-based replication model with quorum commit** to guarantee consistency and fault tolerance.

Each node maintains a local SQLite database, and replication ensures that all nodes eventually converge to the same ledger state.

---

## 2. Replication Strategy

### Approach: Leader-Based Replication (Passive Replication)

The system follows a **primary-backup (leader-follower)** model:

- A single node is elected as the **leader** using ZooKeeper.
- All client write requests (`POST /pay`) are handled by the leader.
- The leader is responsible for:
  - creating log entries
  - replicating them to follower nodes
  - deciding when entries are committed

Follower nodes:
- do not accept external write requests
- only accept replication messages from the leader

### Workflow

1. Client sends a payment request to a node.
2. If the node is not the leader, it redirects the client to the current leader.
3. The leader:
   - validates the request
   - creates a `PENDING` log entry
   - stores it locally
4. The leader sends an append request to followers.
5. Followers:
   - validate the request
   - store the entry as `PENDING`
   - send acknowledgment (ACK)
6. When a **majority (quorum)** of nodes acknowledge:
   - the leader marks the entry as `COMMITTED`
7. The leader sends a commit request to followers.
8. Followers update the entry from `PENDING` → `COMMITTED`.

---

## 3. Consistency Model

### Model: Strong Consistency (for committed entries)

The system ensures that:

- A payment is considered **final only after quorum commit**.
- All committed entries are consistent across a majority of nodes.

### States

- `PENDING` → entry exists but not yet committed
- `COMMITTED` → entry is durable and agreed by quorum
- `FAILED` → entry could not be processed

### Guarantees

- No committed transaction is lost after quorum is reached.
- Duplicate committed entries are prevented.
- Clients can trust committed data as the source of truth.

---

## 4. Quorum Mechanism

The system uses **majority-based quorum** for replication.

For a cluster of N nodes:

- Write quorum = ⌊N/2⌋ + 1

Example (3 nodes):
- Leader + 1 follower = quorum (2/3)

### Behavior

- Leader waits for acknowledgements from followers.
- If quorum is reached:
  - entry is committed
- If quorum is not reached:
  - entry remains `PENDING` or fails

---

## 5. Data Model

The replicated unit is a **LogEntry**, which represents a payment record.

### Key Fields

- `log_index` → ordering of entries
- `term` → leader epoch (used for coordination)
- `payment_id` → unique identifier (for deduplication)
- `amount`, `currency` → payment data
- `status` → replication state (`PENDING`, `COMMITTED`, `FAILED`)
- `leader_id` → source of the entry
- `physical_time`, `logical_time` → reserved for time synchronization

---

## 6. Replication Protocol

The system defines internal communication messages between nodes:

### Append Entries

- `AppendEntriesRequest`
- `AppendEntriesResponse`

Used for:
- replicating new log entries from leader to followers

---

### Commit Entries

- `CommitRequest`
- `CommitResponse`

Used for:
- informing followers that an entry has been committed

---

### Catch-Up (Future Use)

- `CatchUpRequest`
- `CatchUpResponse`

Reserved for:
- node recovery
- synchronization after failures

---

## 7. Deduplication Strategy

Each payment includes a unique `payment_id`.

### Rules

- If a request with the same `payment_id` is received:
  - system checks existing records
  - returns existing result instead of creating a new entry

### Purpose

- prevent duplicate transactions
- handle client retries and network failures safely

---

## 8. Failure Handling (Overview)

- If a follower fails:
  - system can still commit using quorum
- If the leader fails:
  - a new leader is elected using ZooKeeper
- Pending entries may be reprocessed or handled by recovery logic (Member 1)

---

## 9. Trade-offs

### Advantages

- Strong consistency for financial transactions
- Fault tolerance through replication
- Clear separation of leader and follower responsibilities

### Limitations

- Increased latency due to quorum requirement
- Higher storage usage (replicated data on all nodes)
- Temporary unavailability if leader fails before re-election

---

## 10. Summary

This design ensures that:

- payment data is safely replicated across nodes
- consistency is maintained through quorum-based commit
- duplicate transactions are prevented
- the system remains fault-tolerant and reliable

This approach is well-suited for financial systems where **data correctness is more important than low latency**.

## 11. Implementation Status

### Storage layer

- `SQLiteStore` now includes `AppendPending`, `CommitByPaymentID`, `GetPaymentByID`, `ExistsByPaymentID`, and `ListCommittedAfter`.
- Sentinel errors added: `ErrDuplicatePaymentID` and `ErrPaymentNotFound`.

### Replication protocol

- `CommitRequest` now carries both `payment_id` and `log_index`.
- `CommitRequest.Validate()` now requires `payment_id` (in addition to existing index checks).

### Replication client

- `HTTPClient` implemented with `AppendToFollower` and `CommitToFollower` for leader-to-follower replication calls.

### Replication service

- `ReplicateWithQuorum` implemented end-to-end: local append, follower append fan-out, quorum decision, local commit on quorum, and follower commit fan-out (best effort).
- Service contracts are defined via `LocalLedger` and `FollowerTransport` interfaces.

### Coordination accessors

- `Manager` now exposes `GetFollowerURLs()`, `AdvanceLogHead(nextIndex int64)`, and `CurrentLogHead()` for replication flow.
- These accessors support replication index/follower discovery without changing election logic.

### API layer

- `Coordinator` and `Replicator` interfaces are wired in `internal/api` for handler-level orchestration.
- `POST /pay` leader flow is implemented: leader check/redirect, dedup check, log index assignment, quorum replication call, and response handling.
- Follower replication endpoints are implemented: `POST /internal/append` and `POST /internal/commit`.

### Entrypoint

- `cmd/quorapay-node/main.go` now wires replication by constructing `replication.NewHTTPClient(nil)` and `replication.NewReplicationService(store, replClient)`, then passing the service into `api.NewHandler(...)`.

### Tests

- `internal/storage/sqlite_test.go`: append/duplicate/commit/get/exists behavior for persistence.
- `internal/replication/client_test.go`: HTTP replication client success/error/timeout behavior.
- `internal/replication/service_test.go`: quorum service behavior across local append, quorum success/failure, commit, and follower commit fan-out.
- `internal/api/internal_handlers_test.go`: follower endpoint behavior for `/internal/append` and `/internal/commit`.
- `internal/api/pay_handler_test.go`: `POST /pay` redirect, no-leader, dedup, quorum success/failure, and invalid-body cases.

### What remains

- PR 4 catch-up path is still pending coordination with Member 1: `GET /internal/catchup` endpoint and related sync handler flow are not implemented yet.