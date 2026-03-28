#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PID_DIR="$ROOT_DIR/data/pids"

. "$ROOT_DIR/scripts/lib/cluster_env.sh"

extract_field() {
	echo "$1" | sed -n "s/.*\"$2\":\"\\([^\"]*\\)\".*/\\1/p"
}

for spec in $(echo "$NODES" | tr ',' ' '); do
	port=$(echo "$spec" | cut -d: -f2)
	response=$(curl -fsS "http://localhost:$port/status" 2>/dev/null || true)
	if [ -z "$response" ]; then
		continue
	fi

	role=$(extract_field "$response" "role")
	if [ "$role" != "LEADER" ]; then
		continue
	fi

	node_id=$(extract_field "$response" "node_id")
	pid_file="$PID_DIR/node-$node_id.pid"
	if [ ! -f "$pid_file" ]; then
		echo "leader identified as node $node_id, but pid file is missing" >&2
		exit 1
	fi

	pid=$(cat "$pid_file")
	if [ -z "$pid" ] || ! kill -0 "$pid" >/dev/null 2>&1; then
		echo "leader identified as node $node_id, but pid $pid is not running" >&2
		rm -f "$pid_file"
		exit 1
	fi

	kill "$pid"
	rm -f "$pid_file"
	echo "killed leader node $node_id (pid $pid)"
	exit 0
done

echo "no reachable leader found in NODES='$NODES'" >&2
exit 1
