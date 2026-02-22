# quorapay-node

## Overview
Main executable entrypoint for the Quorapay node service. The same binary is used for all nodes, with differences provided by `NODE_ID`, `PORT`, `ZK_ADDR`, and `STORAGE_PATH`.

## Contents
- Node process bootstrap and lifecycle initialization.
- Configuration binding for environment and file inputs.
- Wiring of API, coordination, replication, and storage modules.
- Startup logging and graceful shutdown hooks.

## Not in Scope
- Reusable domain logic that belongs in `internal/`.
- Deployment manifests or orchestration scripts.
- Client-facing CLI or web code.

## Ownership
Owner: shared | Branch: shared
