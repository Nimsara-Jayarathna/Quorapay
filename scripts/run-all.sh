#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

printf '%s\n' 'Starting Quorapay stack (nodes + admin + gateway)...'

printf '%s\n' '[1/4] Checking ZooKeeper...'
"$ROOT_DIR/scripts/run-zk.sh"
printf '%s\n' '[2/4] Starting nodes...'
"$ROOT_DIR/scripts/run-nodes.sh"
printf '%s\n' '[3/4] Starting admin...'
"$ROOT_DIR/scripts/run-admin.sh"
printf '%s\n' '[4/4] Starting gateway...'
"$ROOT_DIR/scripts/run-gateway.sh"

printf '%s\n' 'Quorapay stack started.'
printf '%s\n' 'Health checks:'

curl -sS http://localhost:18090/health >/dev/null 2>&1 && echo 'admin: ok' || echo 'admin: not reachable'
curl -sS http://localhost:18100/health >/dev/null 2>&1 && echo 'gateway: ok' || echo 'gateway: not reachable'
