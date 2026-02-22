# storage

## Overview
Local ledger persistence abstractions and durability boundaries for each node. The same storage module is used across instances, with node-specific data paths set by `STORAGE_PATH` alongside `NODE_ID` and `PORT`.

## Contents
- Ledger append/read interfaces.
- Durability and flush behavior abstractions.
- Storage adapter implementations.
- Recovery and replay support helpers.

## Not in Scope
- ZooKeeper coordination behavior.
- Network API handler wiring.
- Deployment-specific volume configuration.

## Ownership
Owner: shared | Branch: shared
