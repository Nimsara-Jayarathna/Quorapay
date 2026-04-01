#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

printf '%s\n' 'Stopping Quorapay stack (gateway + admin + nodes)...'

"$ROOT_DIR/scripts/stop-gateway.sh" || true
"$ROOT_DIR/scripts/stop-admin.sh" || true
"$ROOT_DIR/scripts/stop-nodes.sh" || true

printf '%s\n' 'Quorapay stack stopped.'
