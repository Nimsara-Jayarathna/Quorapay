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

Variables:

- `VITE_NODE_URLS` optional comma-separated list of node base URLs.
- `VITE_CLUSTER_SIZE` optional generated node count when `VITE_NODE_URLS` is not set.
- `VITE_NODE_BASE_PORT` optional generated base port when `VITE_NODE_URLS` is not set.
- `VITE_NODE_HOST` optional generated host when `VITE_NODE_URLS` is not set.
- `VITE_DEFAULT_NODE_INDEX` default selected node index.

Recommended mapping with backend:
- If backend uses `CLUSTER_SIZE=3|5|7`, set `VITE_CLUSTER_SIZE` to the same value.

Example:

```env
VITE_NODE_URLS="http://localhost:8001,http://localhost:8002,http://localhost:8003"
VITE_DEFAULT_NODE_INDEX=0
```

Auto-generated topology example (no explicit list):

```env
VITE_CLUSTER_SIZE=5
VITE_NODE_BASE_PORT=8001
VITE_NODE_HOST=localhost
VITE_DEFAULT_NODE_INDEX=0
```

7-node UI example:

```env
VITE_CLUSTER_SIZE=7
VITE_NODE_BASE_PORT=8001
VITE_NODE_HOST=localhost
VITE_DEFAULT_NODE_INDEX=0
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
- Node selector populated from environment URLs or generated from cluster env values.
- Automatic topology discovery from `GET /cluster/nodes` so UI adapts when active node count changes.
- Node status fetch from `GET /status`.
- Local-only node termination via `POST /admin/shutdown` for failover testing.
- Payment submission to `POST /pay` with generated UUID support.
- Ledger viewer from `GET /ledger` with status filter.
- Clear unreachable-node and API error states.

## Notes
- This project does not mock backend behavior.
- Endpoints are called directly against running Quorapay node services.
- In the current baseline milestone, `/status`, `/ledger`, and `/admin/shutdown` are available. `POST /pay` is still a future milestone and will return an error until payment processing is implemented.
