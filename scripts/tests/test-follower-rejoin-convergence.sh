#!/bin/sh

set -eu

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
	json=$(curl -fsS "http://localhost:$1/ledger")
	extract_number "$json" "count"
}

find_leader_and_follower_port() {
	leader_port=""
	follower_port=""
	for port in 8001 8002 8003; do
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

wait_counts_match() {
	target="$1"
	attempts=0
	while [ "$attempts" -lt 60 ]; do
		ok=1
		for port in 8001 8002 8003; do
			count=$(ledger_count_for_port "$port" || echo -1)
			if [ "$count" != "$target" ]; then
				ok=0
			fi
		done
		[ "$ok" -eq 1 ] && return 0
		attempts=$((attempts + 1))
		sleep 1
	done
	return 1
}

submit_payment() {
	port="$1"
	id="$2"
	curl -fsS -X POST "http://localhost:$port/pay" \
		-H "Content-Type: application/json" \
		-d "{\"payment_id\":\"$id\",\"amount\":10,\"currency\":\"USD\"}" >/dev/null
}

pair=$(find_leader_and_follower_port) || {
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
	exit 1
}

echo "PASS: follower crash + rejoin converged. final count=$leader_count"
