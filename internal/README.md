# internal

## Overview
Reusable node service modules for Quorapay internals. All node instances share one codebase, with per-instance behavior configured by `NODE_ID`, `PORT`, and `STORAGE_PATH` at runtime.

## Contents
- API layer and request handling modules.
- ZooKeeper coordination and leadership helpers.
- Replication, storage, and ordering utilities.
- Consensus semantics for safe commit behavior.

## Not in Scope
- Executable entrypoints (`cmd/`).
- Client CLI or web application code.
- Deployment and environment manifests.

## Ownership
Owner: shared | Branch: shared
