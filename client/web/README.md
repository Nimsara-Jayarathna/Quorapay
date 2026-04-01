# Quorapay Web Client

## Overview
React + Vite + TypeScript web client for the Quorapay prototype. This UI targets demo workflows: selecting a node, checking node status, submitting payments, and viewing ledger entries.

## Tech Stack
- React
- Vite
- TypeScript
- Tailwind CSS
- Fetch API

## Environment
Create `.env` from `.env.example`:

```bash
cp .env.example .env
```

Variables (required set used by the app):

- `VITE_SEED_NODE_URL` seed URL used first for cluster discovery.
- `VITE_NODE_HOST` host used for incremental fallback scan.
- `VITE_NODE_BASE_PORT` start port used for incremental fallback scan.
- `VITE_NODE_PORT_STEP` port step size for incremental fallback scan.
- `VITE_NODE_SCAN_COUNT` number of ports to probe in fallback scan.
- `VITE_DEFAULT_NODE_INDEX` default selected node index.
- `VITE_BLOCKING_MODAL_TIMEOUT_MS` auto-close timeout (ms) for non-loading payment blocking modal states.
- `VITE_PAYMENT_MODAL_MANUAL_CLOSE_ENABLED` modal close mode: `true` = manual close only (no auto-timeout), `false` = timeout auto-close only (no Close button).
- `VITE_INLINE_MESSAGE_TIMEOUT_MS` auto-dismiss timeout (ms) for inline error/info banners in client/admin views.
- `VITE_ADMIN_API_BASE_URL` base URL for separate admin service (`/admin/node/{id}/{action}`).

Example:

```env
VITE_SEED_NODE_URL=http://localhost:8001
VITE_NODE_HOST=localhost
VITE_NODE_BASE_PORT=8001
VITE_NODE_PORT_STEP=1
VITE_NODE_SCAN_COUNT=7
VITE_DEFAULT_NODE_INDEX=0
VITE_BLOCKING_MODAL_TIMEOUT_MS=1750
VITE_PAYMENT_MODAL_MANUAL_CLOSE_ENABLED=true
VITE_INLINE_MESSAGE_TIMEOUT_MS=4500
VITE_ADMIN_API_BASE_URL=http://localhost:18090
VITE_GATEWAY_API_BASE_URL=http://localhost:18100
```

## Run

```bash
cd client/web
npm install
npm run dev
```

Build for production preview:

```bash
npm run build
npm run preview
```

## Features
- Node selector with automatic cluster discovery via `GET /cluster/nodes`.
- Incremental fallback scan based on host/port env values when discovery seed fails.
- Node status fetch from `GET /status`.
- Node lifecycle control via separate admin service (`POST /admin/node/{id}/{start|stop|restart}`) with bearer token.
- Payment submission to `POST /pay` with generated UUID support.
- Ledger viewer from `GET /ledger` with status filter.
- Clear unreachable-node and API error states.

## Notes
- This project does not mock backend behavior.
- Endpoints are called directly against running Quorapay services.
- Node business APIs and admin-control APIs are intentionally separated.
