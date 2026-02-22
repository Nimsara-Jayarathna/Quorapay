# consensus

## Overview
Consensus-related semantics around leader role, terms, and safe commit boundaries using ZooKeeper coordination. This module supports one shared node codebase run per instance with `NODE_ID`, `PORT`, and `STORAGE_PATH`.

## Contents
- Leadership term and role state semantics.
- Safety checks for commit eligibility.
- Coordination-to-replication decision boundaries.
- Fencing and stale leader prevention rules.

## Not in Scope
- Third-party consensus library integration.
- Direct ledger file IO primitives.
- Frontend or CLI workflow code.

## Ownership
Owner: shared | Branch: shared
