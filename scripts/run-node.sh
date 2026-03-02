#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PID_DIR="$ROOT_DIR/data/pids"
LOG_DIR="$ROOT_DIR/data/logs"
GO_CACHE_DIR="$ROOT_DIR/data/.gocache"
ENV_FILE="$ROOT_DIR/.env"

if [ $# -ne 1 ]; then
	echo "usage: ./scripts/run-node.sh <A|B|C>" >&2
	exit 1
fi

NODE_ID="$1"
case "$NODE_ID" in
	A|a)
		NODE_ID="A"
		PORT="8001"
		STORAGE_PATH="./data/nodeA/ledger.db"
		;;
	B|b)
		NODE_ID="B"
		PORT="8002"
		STORAGE_PATH="./data/nodeB/ledger.db"
		;;
	C|c)
		NODE_ID="C"
		PORT="8003"
		STORAGE_PATH="./data/nodeC/ledger.db"
		;;
	*)
		echo "unknown node '$NODE_ID'; expected A, B, or C" >&2
		exit 1
		;;
esac

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
