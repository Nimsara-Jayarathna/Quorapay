#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PID_FILE="$ROOT_DIR/data/pids/admin.pid"

if [ ! -f "$PID_FILE" ]; then
	echo "admin service is not running"
	exit 0
fi

pid=$(cat "$PID_FILE")
if [ -n "$pid" ] && kill -0 "$pid" >/dev/null 2>&1; then
	kill "$pid" >/dev/null 2>&1 || true
	sleep 1
	if kill -0 "$pid" >/dev/null 2>&1; then
		kill -9 "$pid" >/dev/null 2>&1 || true
	fi
fi
rm -f "$PID_FILE"
echo "stopped admin service"

