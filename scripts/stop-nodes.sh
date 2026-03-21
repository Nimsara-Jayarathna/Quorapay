#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PID_DIR="$ROOT_DIR/data/pids"
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

if [ -d "$PID_DIR" ]; then
	for pid_file in "$PID_DIR"/*.pid; do
		if [ ! -f "$pid_file" ]; then
			continue
		fi

		pid=$(cat "$pid_file")
		terminate_pid "$pid"
		rm -f "$pid_file"
	done
fi

if command -v lsof >/dev/null 2>&1; then
	for port in 8001 8002 8003; do
		for pid in $(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null); do
			terminate_pid "$pid"
		done
	done
fi

if [ "$STOPPED" -eq 1 ]; then
	echo "stopped node processes"
fi
