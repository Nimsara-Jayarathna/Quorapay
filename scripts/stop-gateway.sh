#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PID_FILE="$ROOT_DIR/data/pids/gateway.pid"

if [ ! -f "$PID_FILE" ]; then
	echo "gateway is not running"
	exit 0
fi

pid=$(cat "$PID_FILE")
if [ -n "$pid" ] && kill -0 "$pid" >/dev/null 2>&1; then
	kill "$pid" || true
fi
rm -f "$PID_FILE"
echo "stopped gateway"
