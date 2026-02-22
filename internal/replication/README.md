# replication

## Overview
Leader-to-follower replication protocol components, including append, commit, and synchronization behavior. Modules here are reused by all node instances configured by `NODE_ID`, `PORT`, and `STORAGE_PATH`.

## Contents
- Append and acknowledgement handling logic.
- Quorum commit progression rules.
- Catch-up and resynchronization workflows.
- Replication state tracking structures.

## Not in Scope
- External payment provider integrations.
- ZooKeeper membership/session wrappers.
- Client UI or CLI command implementation.

## Ownership
Owner: shared | Branch: shared
