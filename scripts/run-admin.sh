#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PID_DIR="$ROOT_DIR/data/pids"
LOG_DIR="$ROOT_DIR/data/logs"
GO_CACHE_DIR="$ROOT_DIR/data/.gocache"
BIN_DIR="$ROOT_DIR/data/bin"
ADMIN_BIN="$BIN_DIR/quorapay-admin"
COMMON_ENV_FILE="${COMMON_ENV_FILE:-$ROOT_DIR/.env.common}"
ADMIN_ENV_FILE="${ADMIN_ENV_FILE:-$ROOT_DIR/.env.admin}"

mkdir -p "$PID_DIR" "$LOG_DIR" "$GO_CACHE_DIR" "$BIN_DIR"

if ! command -v go >/dev/null 2>&1; then
	echo "go is required but was not found on PATH" >&2
	exit 1
fi

# Load shared runtime env first (cluster topology like CLUSTER_SIZE/NODES),
# then override with admin-only env values.
if [ -f "$COMMON_ENV_FILE" ]; then
	set -a
	# shellcheck disable=SC1090
	. "$COMMON_ENV_FILE"
	set +a
fi

if [ -f "$ADMIN_ENV_FILE" ]; then
	set -a
	# shellcheck disable=SC1090
	. "$ADMIN_ENV_FILE"
	set +a
fi

ADMIN_PORT="${ADMIN_PORT:-8090}"
PID_FILE="$PID_DIR/admin.pid"
LOG_FILE="$LOG_DIR/admin.log"

if [ -f "$PID_FILE" ]; then
	existing_pid=$(cat "$PID_FILE")
	if [ -n "$existing_pid" ] && kill -0 "$existing_pid" >/dev/null 2>&1; then
		echo "admin service is already running (pid $existing_pid)" >&2
		exit 1
	fi
	rm -f "$PID_FILE"
fi

cd "$ROOT_DIR"
env GOCACHE="$GO_CACHE_DIR" go build -o "$ADMIN_BIN" ./cmd/quorapay-admin

nohup env \
	GOCACHE="$GO_CACHE_DIR" \
	CLUSTER_SIZE="${CLUSTER_SIZE:-}" \
	CLUSTER_BASE_PORT="${CLUSTER_BASE_PORT:-}" \
	NODES="${NODES:-}" \
	ADMIN_PORT="$ADMIN_PORT" \
	ADMIN_API_TOKEN="${ADMIN_API_TOKEN:-}" \
	ADMIN_CORS_ALLOWED_ORIGINS="${ADMIN_CORS_ALLOWED_ORIGINS:-http://localhost:5173}" \
	ADMIN_SCRIPT_ROOT="${ADMIN_SCRIPT_ROOT:-.}" \
	RUN_NODE_SCRIPT="${RUN_NODE_SCRIPT:-./scripts/run-node.sh}" \
	KILL_NODE_SCRIPT="${KILL_NODE_SCRIPT:-./scripts/kill-node.sh}" \
	ZK_ADDR="${ZK_ADDR:-localhost:2181}" \
	"$ADMIN_BIN" >"$LOG_FILE" 2>&1 &

echo $! >"$PID_FILE"
echo "started admin service on port $ADMIN_PORT (pid $(cat "$PID_FILE")) env=$ADMIN_ENV_FILE"
