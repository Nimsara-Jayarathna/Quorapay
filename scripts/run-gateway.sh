#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PID_DIR="$ROOT_DIR/data/pids"
LOG_DIR="$ROOT_DIR/data/logs"
GO_CACHE_DIR="$ROOT_DIR/data/.gocache"
BIN_DIR="$ROOT_DIR/data/bin"
GATEWAY_BIN="$BIN_DIR/quorapay-gateway"
. "$ROOT_DIR/scripts/lib/cluster_env.sh"

mkdir -p "$PID_DIR" "$LOG_DIR" "$GO_CACHE_DIR" "$BIN_DIR"

if ! command -v go >/dev/null 2>&1; then
	echo "go is required but was not found on PATH" >&2
	exit 1
fi

SETSID_BIN=""
if command -v setsid >/dev/null 2>&1; then
	SETSID_BIN="$(command -v setsid)"
fi

GATEWAY_PORT="${GATEWAY_PORT:-18100}"
GATEWAY_CORS_ALLOWED_ORIGINS="${GATEWAY_CORS_ALLOWED_ORIGINS:-http://localhost:5173}"
PID_FILE="$PID_DIR/gateway.pid"
LOG_FILE="$LOG_DIR/gateway.log"

if [ -f "$PID_FILE" ]; then
	existing_pid=$(cat "$PID_FILE")
	if [ -n "$existing_pid" ] && kill -0 "$existing_pid" >/dev/null 2>&1; then
		echo "gateway is already running (pid $existing_pid)" >&2
		exit 1
	fi
	rm -f "$PID_FILE"
fi

cd "$ROOT_DIR"
env GOCACHE="$GO_CACHE_DIR" go build -o "$GATEWAY_BIN" ./cmd/quorapay-gateway

nohup env \
	GOCACHE="$GO_CACHE_DIR" \
	NODES="$NODES" \
	CLUSTER_SIZE="${CLUSTER_SIZE:-3}" \
	CLUSTER_BASE_PORT="${CLUSTER_BASE_PORT:-8001}" \
	GATEWAY_PORT="$GATEWAY_PORT" \
	GATEWAY_CORS_ALLOWED_ORIGINS="$GATEWAY_CORS_ALLOWED_ORIGINS" \
	${SETSID_BIN:-} "$GATEWAY_BIN" >"$LOG_FILE" 2>&1 &

echo $! >"$PID_FILE"
pid=$(cat "$PID_FILE")
for _ in 1 2 3 4 5 6 7 8 9 10; do
	if curl -sS "http://localhost:$GATEWAY_PORT/health" >/dev/null 2>&1; then
		echo "started gateway on port $GATEWAY_PORT (pid $pid)"
		exit 0
	fi
	if ! kill -0 "$pid" >/dev/null 2>&1; then
		break
	fi
	sleep 0.3
done

echo "gateway failed to stay up on port $GATEWAY_PORT (pid $pid)" >&2
tail -n 50 "$LOG_FILE" >&2 || true
rm -f "$PID_FILE"
exit 1
