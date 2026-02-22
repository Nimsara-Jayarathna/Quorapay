# api

## Overview
Node-facing API handlers, routing, and request/response models. These modules are reused by the shared node binary configured via `NODE_ID`, `PORT`, and `STORAGE_PATH`.

## Contents
- HTTP handler definitions.
- Request and response schema structures.
- Route registration and middleware glue.
- Input validation and API error mapping.

## Not in Scope
- Replication algorithm state machines.
- ZooKeeper session and election logic.
- Client-side UI rendering code.

## Ownership
Owner: shared | Branch: shared
