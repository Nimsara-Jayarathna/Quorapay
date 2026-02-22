# cmd

## Overview
Executable entrypoints for Quorapay services. Node instances share one codebase and are parameterized at runtime using `NODE_ID`, `PORT`, and `STORAGE_PATH`.

## Contents
- Service binary entrypoint directories.
- Process startup wiring and bootstrap code.
- Runtime configuration loading for executable startup.

## Not in Scope
- Reusable business logic modules.
- Protocol and storage implementations.
- Client UI or CLI features.

## Ownership
Owner: shared | Branch: shared
