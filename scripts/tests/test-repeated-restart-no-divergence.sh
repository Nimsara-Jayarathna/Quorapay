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

wait_counts_match_all() {
	attempts=0
	while [ "$attempts" -lt 60 ]; do
		c1=$(ledger_count_for_port 8001 || echo -1)
		c2=$(ledger_count_for_port 8002 || echo -1)
		c3=$(ledger_count_for_port 8003 || echo -1)
		if [ "$c1" = "$c2" ] && [ "$c2" = "$c3" ] && [ "$c1" != "-1" ]; then
			echo "$c1"
			return 0
		fi
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

cycles="${1:-3}"
cycle=1
while [ "$cycle" -le "$cycles" ]; do
	pair=$(find_leader_and_follower_port) || {
		echo "FAIL: could not determine leader/follower at cycle $cycle" >&2
		exit 1
	}
	leader_port=$(echo "$pair" | awk '{print $1}')
	follower_port=$(echo "$pair" | awk '{print $2}')
	follower_json=$(status_for_port "$follower_port")
	follower_id=$(extract_string "$follower_json" "node_id")

	echo "cycle=$cycle leader=$leader_port follower=$follower_port node=$follower_id"

	curl -fsS -X POST "http://localhost:$follower_port/admin/shutdown" >/dev/null
	sleep 1
	submit_payment "$leader_port" "m1-restart-$cycle-a-$(date +%s)"
	submit_payment "$leader_port" "m1-restart-$cycle-b-$(date +%s)"
	./scripts/run-node.sh "$follower_id" >/dev/null
	sleep 2

	wait_counts_match_all >/dev/null || {
		echo "FAIL: ledger counts diverged after cycle $cycle" >&2
		exit 1
	}

	cycle=$((cycle + 1))
done

final_count=$(wait_counts_match_all) || {
	echo "FAIL: final ledger counts diverged" >&2
	exit 1
}

echo "PASS: repeated restart test converged across all nodes, final count=$final_count"
