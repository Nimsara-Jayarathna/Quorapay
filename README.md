# Quorapay

Quorapay is a distributed payment prototype with leader-based replication, ZooKeeper coordination, and a replicated ledger.

## Repository Layout
- `client/web/` React web client.
- `cmd/quorapay-node/` node service binary.
- `cmd/quorapay-admin/` separate admin control service binary.
- `internal/` backend modules (API, coordination, replication, storage, time sync, admin).
- `scripts/` local lifecycle + test scripts.
- `data/` runtime artifacts (logs/pids/sqlite), git-ignored.

## Environment Model
Quorapay now uses split env files at repo root:

- `.env.common` shared topology + coordination
- `.env.node` node runtime settings
- `.env.admin` admin service settings

Templates:

- `.env.common.example`
- `.env.node.example`
- `.env.admin.example`

## Quick Start
1. Configure env files (copy from examples if needed).
2. Start ZooKeeper readiness check:

```bash
./scripts/run-zk.sh
```

3. Start cluster nodes:

```bash
./scripts/run-nodes.sh
```

4. Start separate admin service:

```bash
./scripts/run-admin.sh
```

5. Verify status:

```bash
curl -s http://localhost:8001/status
curl -s http://localhost:8002/status
curl -s http://localhost:8003/status
curl -s http://localhost:18090/health
```

6. Stop services:

```bash
./scripts/stop-admin.sh
./scripts/stop-nodes.sh
```

## Node Lifecycle Control (Separate Admin Service)
Admin APIs are served by `quorapay-admin` (not node APIs):

- `POST /admin/node/{id}/start`
- `POST /admin/node/{id}/stop`
- `POST /admin/node/{id}/restart`

Example:

```bash
curl -X POST http://localhost:18090/admin/node/B/stop \
  -H "Authorization: Bearer dev-admin-token"
```

## Web Client
Run web client:

```bash
cd client/web
cp .env.example .env
npm install
npm run dev
```

Build web client:

```bash
npm run build
```

Web env uses only FE variables (seed URL + scan settings + admin API base URL). See `client/web/README.md`.

## Notes
- Business node API and admin-control API are intentionally separated.
- Topology should be defined in `.env.common` (no need for inline `CLUSTER_SIZE=...` commands).
- Detailed script behavior and test commands are documented in `scripts/README.md`.
