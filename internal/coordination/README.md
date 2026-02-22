# coordination

## Overview
ZooKeeper coordination wrappers for membership, leader watches, and election helpers. This logic serves the shared node codebase run with `NODE_ID`, `PORT`, and `STORAGE_PATH` per instance.

## Contents
- ZooKeeper client integration points.
- Membership and liveness watchers.
- Leader election helper utilities.
- Coordination event translation for node runtime.

## Not in Scope
- Ledger persistence and file format handling.
- Client transport endpoint implementations.
- Third-party consensus framework integration.

## Ownership
Owner: shared | Branch: shared
