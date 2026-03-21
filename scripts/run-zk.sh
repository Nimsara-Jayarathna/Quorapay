#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ENV_FILE="$ROOT_DIR/.env"
ZK_ADDR="${ZK_ADDR:-localhost:2181}"

if [ -f "$ENV_FILE" ]; then
	set -a
	# shellcheck disable=SC1090
	. "$ENV_FILE"
	set +a
fi

if ! command -v lsof >/dev/null 2>&1; then
	echo "lsof is required to check ZooKeeper readiness" >&2
	exit 1
fi

host=$(printf '%s' "$ZK_ADDR" | awk -F: '{print $1}')
port=$(printf '%s' "$ZK_ADDR" | awk -F: '{print $2}')

if [ -z "$port" ]; then
	echo "invalid ZK_ADDR: $ZK_ADDR" >&2
	exit 1
fi

if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
	echo "ZooKeeper appears to be running on $host:$port"
	exit 0
fi

echo "ZooKeeper is not listening on $host:$port." >&2
echo "Start your local ZooKeeper service manually, then rerun this check." >&2
exit 1
