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

- `VITE_NODE_URLS` comma-separated list of node base URLs.
- `VITE_DEFAULT_NODE_INDEX` default selected node index.

Example:

```env
VITE_NODE_URLS="http://localhost:8001,http://localhost:8002,http://localhost:8003"
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
- Node selector populated from environment URLs.
- Node status fetch from `GET /status`.
- Payment submission to `POST /pay` with generated UUID support.
- Ledger viewer from `GET /ledger` with status filter.
- Clear unreachable-node and API error states.

## Notes
- This project does not mock backend behavior.
- Endpoints are called directly against running Quorapay node services.
