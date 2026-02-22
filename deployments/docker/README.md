# docker

## Overview
Docker-based local orchestration for ZooKeeper and multiple Quorapay nodes. Containers run the same node image with different `NODE_ID`, `PORT`, and `STORAGE_PATH` values.

## Contents
- `docker-compose` definitions for local clusters.
- Container environment variable mappings.
- Volume and network defaults for demos.
- Startup ordering notes for repeatable runs.

## Not in Scope
- Production container hardening and scaling.
- Application business and protocol logic.
- CI/CD pipeline definitions.

## Ownership
Owner: shared | Branch: shared
