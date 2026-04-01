#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PID_DIR="$ROOT_DIR/data/pids"

. "$ROOT_DIR/scripts/lib/cluster_env.sh"

if [ $# -ne 1 ]; then
	echo "usage: ./scripts/kill-node.sh <NODE_ID>" >&2
	exit 1
fi

REQ_NODE_ID=$(echo "$1" | tr '[:lower:]' '[:upper:]')
NODE_ID=""
PORT=""
for spec in $(echo "$NODES" | tr ',' ' '); do
	id=$(echo "$spec" | cut -d: -f1)
	port=$(echo "$spec" | cut -d: -f2)
	if [ "$id" = "$REQ_NODE_ID" ]; then
		NODE_ID="$id"
		PORT="$port"
		break
	fi
done

if [ -z "$NODE_ID" ] || [ -z "$PORT" ]; then
	echo "unknown node '$REQ_NODE_ID'; not found in NODES='$NODES'" >&2
	exit 1
fi

PID_FILE="$PID_DIR/node-$NODE_ID.pid"
STOPPED=0

terminate_pid() {
	pid="$1"
	if [ -z "$pid" ]; then
		return
	fi
	if ! kill -0 "$pid" >/dev/null 2>&1; then
		return
	fi
	kill "$pid" >/dev/null 2>&1 || true
	sleep 1
	if kill -0 "$pid" >/dev/null 2>&1; then
		kill -9 "$pid" >/dev/null 2>&1 || true
	fi
	STOPPED=1
}

if [ -f "$PID_FILE" ]; then
	pid=$(cat "$PID_FILE")
	terminate_pid "$pid"
	rm -f "$PID_FILE"
fi

if command -v lsof >/dev/null 2>&1; then
	for pid in $(lsof -tiTCP:"$PORT" -sTCP:LISTEN 2>/dev/null); do
		terminate_pid "$pid"
	done
fi

if [ "$STOPPED" -eq 1 ]; then
	echo "stopped node $NODE_ID"
else
	echo "node $NODE_ID is not running"
fi

