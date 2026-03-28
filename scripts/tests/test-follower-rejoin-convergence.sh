#!/bin/sh

set -eu
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
CONVERGENCE_ATTEMPTS="${CONVERGENCE_ATTEMPTS:-180}"
CONVERGENCE_SLEEP_SECONDS="${CONVERGENCE_SLEEP_SECONDS:-1}"
PAIR_TIMEOUT_SECONDS="${PAIR_TIMEOUT_SECONDS:-90}"

. "$ROOT_DIR/scripts/lib/cluster_env.sh"

extract_string() {
	echo "$1" | sed -n "s/.*\"$2\":\"\\([^\"]*\\)\".*/\\1/p"
}

extract_number() {
	echo "$1" | sed -n "s/.*\"$2\":\\([-0-9][0-9]*\\).*/\\1/p"
}

status_for_port() {
	curl -fsS "http://localhost:$1/status" 2>/dev/null || true
}

ledger_count_for_port() {
	json=$(curl -fsS "http://localhost:$1/ledger" 2>/dev/null || true)
	[ -n "$json" ] || {
		echo -1
		return
	}
	extract_number "$json" "count"
}

find_leader_and_follower_port() {
	leader_port=""
	follower_port=""
	for spec in $(echo "$NODES" | tr ',' ' '); do
		port=$(echo "$spec" | cut -d: -f2)
		json=$(status_for_port "$port")
		[ -n "$json" ] || continue
		role=$(extract_string "$json" "role")
		if [ "$role" = "LEADER" ]; then
			leader_port="$port"
		elif [ -z "$follower_port" ] && [ "$role" = "FOLLOWER" ]; then
			follower_port="$port"
		fi
	done
	[ -n "$leader_port" ] && [ -n "$follower_port" ] || return 1
	echo "$leader_port $follower_port"
}

wait_for_leader_and_follower_port() {
	start_ts=$(date +%s)
	while :; do
		now_ts=$(date +%s)
		if [ $((now_ts - start_ts)) -gt "$PAIR_TIMEOUT_SECONDS" ]; then
			return 1
		fi
		if pair=$(find_leader_and_follower_port); then
			echo "$pair"
			return 0
		fi
		sleep 1
	done
}

wait_counts_match() {
	target="$1"
	attempts=0
	while [ "$attempts" -lt "$CONVERGENCE_ATTEMPTS" ]; do
		ok=1
		for spec in $(echo "$NODES" | tr ',' ' '); do
			port=$(echo "$spec" | cut -d: -f2)
			count=$(ledger_count_for_port "$port" || echo -1)
			if [ "$count" != "$target" ]; then
				ok=0
			fi
		done
		[ "$ok" -eq 1 ] && return 0
		attempts=$((attempts + 1))
		if [ $((attempts % 10)) -eq 0 ]; then
			echo "waiting for convergence... attempt=$attempts target_count=$target"
		fi
		sleep "$CONVERGENCE_SLEEP_SECONDS"
	done
	return 1
}

print_counts() {
	for spec in $(echo "$NODES" | tr ',' ' '); do
		id=$(echo "$spec" | cut -d: -f1)
		port=$(echo "$spec" | cut -d: -f2)
		count=$(ledger_count_for_port "$port" || echo -1)
		echo "node=$id port=$port count=$count"
	done
}

all_counts_match() {
	baseline=""
	for spec in $(echo "$NODES" | tr ',' ' '); do
		port=$(echo "$spec" | cut -d: -f2)
		count=$(ledger_count_for_port "$port" || echo -1)
		if [ "$count" = "-1" ]; then
			return 1
		fi
		if [ -z "$baseline" ]; then
			baseline="$count"
		elif [ "$count" != "$baseline" ]; then
			return 1
		fi
	done
	return 0
}

print_follower_debug() {
	node_id="$1"
	log_file="./data/logs/node-$node_id.log"
	echo "---- follower debug: node=$node_id ----" >&2
	if [ -f "$log_file" ]; then
		echo "last 80 lines of $log_file" >&2
		tail -n 80 "$log_file" >&2 || true
		echo "catch-up/error lines from $log_file" >&2
		grep -E "catch-up|reconcile failed|zookeeper error|fault state transition" "$log_file" | tail -n 40 >&2 || true
	else
		echo "log file not found: $log_file" >&2
	fi
}

submit_payment() {
	port="$1"
	id="$2"
	curl -fsS -X POST "http://localhost:$port/pay" \
		-H "Content-Type: application/json" \
		-d "{\"payment_id\":\"$id\",\"amount\":10,\"currency\":\"USD\"}" >/dev/null
}

pair=$(wait_for_leader_and_follower_port) || {
	echo "FAIL: could not determine leader/follower ports" >&2
	exit 1
}
leader_port=$(echo "$pair" | awk '{print $1}')
follower_port=$(echo "$pair" | awk '{print $2}')

follower_json=$(status_for_port "$follower_port")
follower_id=$(extract_string "$follower_json" "node_id")
[ -n "$follower_id" ] || {
	echo "FAIL: could not read follower node_id from port $follower_port" >&2
	exit 1
}

echo "leader port=$leader_port follower port=$follower_port node_id=$follower_id"

if ! all_counts_match; then
	echo "FAIL: cluster is already divergent before test start; convergence test requires equal baseline counts" >&2
	print_counts >&2
	echo "Hint: run a clean reset before this test (stop nodes, clear local data, start nodes)." >&2
	exit 1
fi

submit_payment "$leader_port" "m1-rejoin-base-$(date +%s)"

echo "Stopping follower node $follower_id..."
curl -fsS -X POST "http://localhost:$follower_port/admin/shutdown" >/dev/null
sleep 2

submit_payment "$leader_port" "m1-rejoin-1-$(date +%s)"
submit_payment "$leader_port" "m1-rejoin-2-$(date +%s)"

echo "Restarting follower node $follower_id..."
./scripts/run-node.sh "$follower_id" >/dev/null
sleep 2

leader_count=$(ledger_count_for_port "$leader_port")
wait_counts_match "$leader_count" || {
	echo "FAIL: ledger counts did not converge to $leader_count after follower rejoin" >&2
	print_counts >&2
	print_follower_debug "$follower_id"
	exit 1
}

echo "PASS: follower crash + rejoin converged. final count=$leader_count"
