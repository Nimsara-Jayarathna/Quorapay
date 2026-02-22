# zookeeper

## Overview
ZooKeeper ensemble configuration and local defaults used for coordination. Node binaries remain shared and consume coordination endpoints alongside `NODE_ID`, `PORT`, and `STORAGE_PATH` settings.

## Contents
- Local ensemble configuration examples.
- Session timeout and election-related defaults.
- Connection endpoint definitions for node processes.
- Notes for reproducible local startup.

## Not in Scope
- Application replication or ledger code.
- Production infrastructure hardening.
- Secret material or access credentials.

## Ownership
Owner: shared | Branch: shared
