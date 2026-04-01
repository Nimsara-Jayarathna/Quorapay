#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PID_DIR="$ROOT_DIR/data/pids"
LOG_DIR="$ROOT_DIR/data/logs"
GO_CACHE_DIR="$ROOT_DIR/data/.gocache"
BIN_DIR="$ROOT_DIR/data/bin"
NODE_BIN="$BIN_DIR/quorapay-node"
. "$ROOT_DIR/scripts/lib/cluster_env.sh"

mkdir -p "$PID_DIR" "$LOG_DIR" "$GO_CACHE_DIR" "$BIN_DIR"

if ! command -v go >/dev/null 2>&1; then
	echo "go is required but was not found on PATH" >&2
	exit 1
fi

ZK_ADDR="${ZK_ADDR:-localhost:2181}"
ZK_ROOT="${ZK_ROOT:-/quorapay}"
CORS_ALLOWED_ORIGINS="${CORS_ALLOWED_ORIGINS:-http://localhost:5173}"

"$ROOT_DIR/scripts/stop-nodes.sh" >/dev/null 2>&1 || true

cd "$ROOT_DIR"
env GOCACHE="$GO_CACHE_DIR" go build -o "$NODE_BIN" ./cmd/quorapay-node

start_node() {
	NODE_ID="$1"
	PORT="$2"
	STORAGE_PATH="$3"
	LOG_FILE="$LOG_DIR/node-$NODE_ID.log"
	PID_FILE="$PID_DIR/node-$NODE_ID.pid"

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
}

for spec in $(echo "$NODES" | tr ',' ' '); do
	NODE_ID=$(echo "$spec" | cut -d: -f1)
	PORT=$(echo "$spec" | cut -d: -f2)
	if [ -z "$NODE_ID" ] || [ -z "$PORT" ]; then
		echo "invalid NODES entry: '$spec' (expected NODE_ID:PORT)" >&2
		exit 1
	fi

	STORAGE_PATH="./data/node$NODE_ID/ledger.db"
	mkdir -p "$ROOT_DIR/data/node$NODE_ID"
	start_node "$NODE_ID" "$PORT" "$STORAGE_PATH"
done
