#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PID_DIR="$ROOT_DIR/data/pids"
LOG_DIR="$ROOT_DIR/data/logs"
GO_CACHE_DIR="$ROOT_DIR/data/.gocache"
ENV_FILE="$ROOT_DIR/.env"

mkdir -p "$ROOT_DIR/data/nodeA" "$ROOT_DIR/data/nodeB" "$ROOT_DIR/data/nodeC" "$PID_DIR" "$LOG_DIR" "$GO_CACHE_DIR"

if ! command -v go >/dev/null 2>&1; then
	echo "go is required but was not found on PATH" >&2
	exit 1
fi

if [ -f "$ENV_FILE" ]; then
	set -a
	# shellcheck disable=SC1090
	. "$ENV_FILE"
	set +a
fi

ZK_ADDR="${ZK_ADDR:-localhost:2181}"
ZK_ROOT="${ZK_ROOT:-/quorapay}"
CORS_ALLOWED_ORIGINS="${CORS_ALLOWED_ORIGINS:-http://localhost:5173}"

"$ROOT_DIR/scripts/stop-nodes.sh" >/dev/null 2>&1 || true

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
		go run ./cmd/quorapay-node >"$LOG_FILE" 2>&1 &

	echo $! >"$PID_FILE"
	echo "started node $NODE_ID on port $PORT (pid $(cat "$PID_FILE"))"
}

cd "$ROOT_DIR"

start_node "A" "8001" "./data/nodeA/ledger.db"
start_node "B" "8002" "./data/nodeB/ledger.db"
start_node "C" "8003" "./data/nodeC/ledger.db"
