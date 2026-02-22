# timesync

## Overview
Timestamp and ordering utilities to support deterministic transaction ordering. These utilities are reused by the shared node codebase configured per instance via `NODE_ID`, `PORT`, and `STORAGE_PATH`.

## Contents
- Timestamp generation and normalization helpers.
- Ordering comparison rules.
- Clock skew handling utilities.
- Time-related validation support.

## Not in Scope
- Network transport implementations.
- ZooKeeper election/session management.
- CLI or web presentation logic.

## Ownership
Owner: shared | Branch: shared
