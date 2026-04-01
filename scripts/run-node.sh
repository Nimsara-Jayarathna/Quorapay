#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PID_DIR="$ROOT_DIR/data/pids"
LOG_DIR="$ROOT_DIR/data/logs"
GO_CACHE_DIR="$ROOT_DIR/data/.gocache"
BIN_DIR="$ROOT_DIR/data/bin"
NODE_BIN="$BIN_DIR/quorapay-node"
. "$ROOT_DIR/scripts/lib/cluster_env.sh"

if [ $# -ne 1 ]; then
	echo "usage: ./scripts/run-node.sh <NODE_ID>" >&2
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
STORAGE_PATH="./data/node$NODE_ID/ledger.db"

mkdir -p "$PID_DIR" "$LOG_DIR" "$GO_CACHE_DIR" "$BIN_DIR"
mkdir -p "$ROOT_DIR/data/node$NODE_ID"

if ! command -v go >/dev/null 2>&1; then
	echo "go is required but was not found on PATH" >&2
	exit 1
fi

ZK_ADDR="${ZK_ADDR:-localhost:2181}"
ZK_ROOT="${ZK_ROOT:-/quorapay}"
CORS_ALLOWED_ORIGINS="${CORS_ALLOWED_ORIGINS:-http://localhost:5173}"
PID_FILE="$PID_DIR/node-$NODE_ID.pid"
LOG_FILE="$LOG_DIR/node-$NODE_ID.log"

if [ -f "$PID_FILE" ]; then
	existing_pid=$(cat "$PID_FILE")
	if [ -n "$existing_pid" ] && kill -0 "$existing_pid" >/dev/null 2>&1; then
		echo "node $NODE_ID is already running (pid $existing_pid)" >&2
		exit 1
	fi
	rm -f "$PID_FILE"
fi

if command -v lsof >/dev/null 2>&1 && lsof -tiTCP:"$PORT" -sTCP:LISTEN >/dev/null 2>&1; then
	echo "port $PORT is already in use; stop the existing process first" >&2
	exit 1
fi

cd "$ROOT_DIR"
env GOCACHE="$GO_CACHE_DIR" go build -o "$NODE_BIN" ./cmd/quorapay-node

nohup env \
	GOCACHE="$GO_CACHE_DIR" \
	NODE_ID="$NODE_ID" \
	PORT="$PORT" \
	BASE_URL="http://localhost:$PORT" \
	CORS_ALLOWED_ORIGINS="$CORS_ALLOWED_ORIGINS" \
	ZK_ADDR="$ZK_ADDR" \
	ZK_ROOT="$ZK_ROOT" \
	STORAGE_PATH="$STORAGE_PATH" \
	"$NODE_BIN" >"$LOG_FILE" 2>&1 &

echo $! >"$PID_FILE"
echo "started node $NODE_ID on port $PORT (pid $(cat "$PID_FILE"))"
