# Quorapay

Quorapay is a distributed payment-processing prototype for e-commerce style flows:
- client starts Stripe Checkout
- cluster leader verifies/finalizes payment
- payment result is replicated with quorum
- replicated ledger is visible from any node

It uses leader-based coordination (ZooKeeper), quorum replication, a separate admin service, and a frontend with Admin and Client routes.

## What The System Includes
- `cmd/quorapay-node`:
  - payment node API (`/pay`, `/ledger`, `/status`, Stripe handlers, internal replication endpoints)
- `cmd/quorapay-gateway`:
  - client-facing payment gateway (`/payments/checkout-session`, `/payments/finalize`, `/payments/cancel`)
  - resolves current leader and forwards requests
- `cmd/quorapay-admin`:
  - node lifecycle control (`start/stop/restart`) via scripts
- `client/web`:
  - `/admin` route (cluster operations, ledger, process logs)
  - `/client` route (Stripe checkout UI, node selector, client payment log)

## Architecture (Current)
1. Client selects an entry node in UI.
2. Client calls gateway for checkout/finalize/cancel.
3. Gateway discovers leader from node `/status` and forwards to leader.
4. Leader processes payment and replicates via quorum.
5. Ledger state converges across nodes; recovered followers catch up from leader.

Replication uses majority quorum:
- `7/4, 6/4, 5/3, 4/3, 3/2, 2/2, 1/1`

## Status Model
Ledger statuses:
- `PENDING`
- `COMMITTED`
- `FAILED`
- `CANCELED`

Process/event stages (examples):
- `RECEIVED`
- `FORWARDED_TO_LEADER`
- `LEADER_PROCESSING`
- `STRIPE_SESSION_CREATED`
- `COMMITTED`
- `REPLICATION_FAILED`
- `QUORUM_NOT_REACHED`
- `STRIPE_CHECKOUT_CANCELED`

## Repo Layout
- `client/web/` frontend
- `cmd/quorapay-node/` node binary
- `cmd/quorapay-gateway/` gateway binary
- `cmd/quorapay-admin/` admin binary
- `internal/` api, coordination, replication, storage, admin internals
- `scripts/` lifecycle and test scripts
- `data/` runtime artifacts (pids, logs, sqlite, bins)

## Prerequisites
- Go installed
- Node.js + npm installed
- ZooKeeper running locally (`localhost:2181`)
- Stripe test secret key (for real test-mode checkout flow)

## Environment Setup
At repo root:
- `.env.common` (cluster topology + ZooKeeper)
- `.env.node` (node runtime + Stripe key)
- `.env.admin` (admin service runtime)

Templates:
- `.env.common.example`
- `.env.node.example`
- `.env.admin.example`

Recommended values:

`.env.common`
```env
CLUSTER_SIZE=7
CLUSTER_BASE_PORT=8001
ZK_ADDR=localhost:2181
ZK_ROOT=/quorapay
```

`.env.node`
```env
CORS_ALLOWED_ORIGINS=http://localhost:5173
SKEW_WARN_MS=300
SKEW_REJECT_MS=500
MAX_MESSAGE_AGE_MS=2000
MAX_FUTURE_DRIFT_MS=500
STRIPE_SECRET_KEY=sk_test_xxx
```

`.env.admin`
```env
ADMIN_PORT=18090
ADMIN_API_TOKEN=dev-admin-token
ADMIN_CORS_ALLOWED_ORIGINS=http://localhost:5173
ADMIN_SCRIPT_ROOT=.
RUN_NODE_SCRIPT=./scripts/run-node.sh
KILL_NODE_SCRIPT=./scripts/kill-node.sh
```

Frontend env is in `client/web/.env` (see `client/web/README.md`).

## Start / Stop
Single-command startup:
```bash
./scripts/run-all.sh
```

Single-command stop:
```bash
./scripts/stop-all.sh
```

Manual startup sequence:
```bash
./scripts/run-zk.sh
./scripts/run-nodes.sh
./scripts/run-admin.sh
./scripts/run-gateway.sh
```

Manual stop sequence:
```bash
./scripts/stop-gateway.sh
./scripts/stop-admin.sh
./scripts/stop-nodes.sh
```

## Run Frontend
```bash
cd client/web
cp .env.example .env
npm install
npm run dev
```

Build:
```bash
npm run build
```

## Core Service Endpoints

### Gateway (`:18100` default)
- `GET /health`
- `POST /payments/checkout-session`
- `POST /payments/finalize`
- `POST /payments/cancel`

### Node (`:8001+`)
- `GET /health`
- `GET /status`
- `GET /ledger`
- `GET /events`
- `POST /pay` (direct/internal test path)
- `POST /stripe/create-checkout-session`
- `GET /stripe/session-status`
- `POST /stripe/finalize-checkout-session`
- `POST /stripe/cancel-checkout-session`
- `GET /cluster/nodes`
- Internal replication:
  - `POST /internal/append`
  - `POST /internal/commit`
  - `POST /internal/cancel`
  - `GET /internal/catchup`

### Admin (`:18090` or configured)
- `GET /health`
- `POST /admin/node/{id}/start`
- `POST /admin/node/{id}/stop`
- `POST /admin/node/{id}/restart`

## Payment Flows

### Client Route (`/client`) - Stripe Path
1. User enters amount/currency and clicks `Pay`.
2. Frontend calls gateway `POST /payments/checkout-session`.
3. Gateway forwards to current leader node.
4. Leader creates Stripe Checkout Session and returns Stripe URL.
5. User pays on Stripe page.
6. On success redirect, frontend calls gateway `POST /payments/finalize`.
7. Leader verifies Stripe session and replicates result with quorum.
8. Ledger shows `COMMITTED` on cluster nodes.

Cancel path:
1. User cancels Stripe checkout.
2. Frontend calls gateway `POST /payments/cancel`.
3. Leader performs replicated cancel path with quorum.
4. Ledger shows `CANCELED` cluster-wide once quorum succeeds.

### Admin Route (`/admin`) - Cluster/Internal Path
- Designed for cluster operations/observability and direct test-mode payment simulation.
- Shows node status, ledger, process logs, and node lifecycle actions.

## Fault Tolerance Behavior
- Follower requests route to leader.
- Gateway retries once with re-resolved leader if forwarding fails.
- Quorum required to finalize/cancel globally.
- If quorum not reached, operation fails with explicit error.
- Rejoined followers catch up committed entries from leader via `/internal/catchup`.

## Stripe Test Notes
- Stripe test mode is supported via `STRIPE_SECRET_KEY`.
- Example success card: `4242 4242 4242 4242`.
- Example decline card: `4000 0000 0000 0002`.
- Decline may remain on Stripe page until user retries/cancels; redirect timing is Stripe-hosted behavior.

## Data, Logs, and Runtime Files
- Node ledgers: `data/node*/ledger.db`
- PID files: `data/pids/*.pid`
- Logs: `data/logs/*.log`
- Built binaries: `data/bin/*`

Useful checks:
```bash
curl -s http://localhost:18100/health
curl -s http://localhost:8001/status
curl -s http://localhost:8001/ledger
tail -n 200 data/logs/gateway.log
tail -n 200 data/logs/node-A.log
```

## Test Scenarios To Run
1. Success flow end-to-end (Stripe -> finalize -> `COMMITTED` on all active nodes).
2. Cancel flow end-to-end (`CANCELED` replicated with quorum).
3. Leader failover during checkout/finalize.
4. Quorum loss and clear failure response.
5. Follower restart and catch-up convergence.
6. Gateway down: client sees popup service-unavailable error.

For script-based automated tests, see:
- `scripts/README.md`
- `scripts/tests/run-all-tests.sh`

## Troubleshooting
- `ERR_CONNECTION_REFUSED` to gateway:
  - ensure `./scripts/run-gateway.sh` is running
  - check `data/logs/gateway.log`
- `no leader available`:
  - check ZooKeeper availability and node `/status`
- Route fallback confusion:
  - only `/admin` and `/client` are valid frontend routes
- Stripe not configured:
  - set `STRIPE_SECRET_KEY` in `.env.node`, restart nodes

## Important Notes
- Event logs (`/events`) are node-local observability.
- Ledger (`/ledger`) is replicated system-of-record.
- For consistency checks, always validate ledger across nodes, not only event stream.

