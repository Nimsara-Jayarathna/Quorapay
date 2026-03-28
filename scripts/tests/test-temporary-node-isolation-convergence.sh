#!/bin/sh

set -eu

ISOLATION_SECONDS="${1:-6}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-90}"
POLL_SECONDS="${POLL_SECONDS:-1}"
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
PID_DIR="$ROOT_DIR/data/pids"

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
		echo "-1"
		return
	}
	extract_number "$json" "count"
}

find_leader_and_follower() {
	leader_port=""
	follower_port=""
	follower_id=""
	for port in 8001 8002 8003; do
		json=$(status_for_port "$port")
		[ -n "$json" ] || continue
		role=$(extract_string "$json" "role")
		if [ "$role" = "LEADER" ]; then
			leader_port="$port"
		elif [ "$role" = "FOLLOWER" ] && [ -z "$follower_port" ]; then
			follower_port="$port"
			follower_id=$(extract_string "$json" "node_id")
		fi
	done
	[ -n "$leader_port" ] && [ -n "$follower_port" ] && [ -n "$follower_id" ] || return 1
	echo "$leader_port $follower_port $follower_id"
}

wait_counts_match_all() {
	start_ts=$(date +%s)
	while :; do
		now_ts=$(date +%s)
		if [ $((now_ts - start_ts)) -gt "$TIMEOUT_SECONDS" ]; then
			return 1
		fi

		c1=$(ledger_count_for_port 8001)
		c2=$(ledger_count_for_port 8002)
		c3=$(ledger_count_for_port 8003)
		if [ "$c1" = "$c2" ] && [ "$c2" = "$c3" ] && [ "$c1" != "-1" ]; then
			echo "$c1"
			return 0
		fi
		sleep "$POLL_SECONDS"
	done
}

submit_payment() {
	port="$1"
	id="$2"
	curl -fsS -X POST "http://localhost:$port/pay" \
		-H "Content-Type: application/json" \
		-d "{\"payment_id\":\"$id\",\"amount\":10,\"currency\":\"USD\"}" >/dev/null
}

echo "Detecting leader + follower for temporary isolation scenario..."
pair=$(find_leader_and_follower) || {
	echo "FAIL: could not determine leader/follower pair" >&2
	exit 1
}
leader_port=$(echo "$pair" | awk '{print $1}')
follower_port=$(echo "$pair" | awk '{print $2}')
follower_id=$(echo "$pair" | awk '{print $3}')
follower_pid_file="$PID_DIR/node-$follower_id.pid"

[ -f "$follower_pid_file" ] || {
	echo "FAIL: follower pid file missing: $follower_pid_file" >&2
	exit 1
}
follower_pid=$(cat "$follower_pid_file")
kill -0 "$follower_pid" >/dev/null 2>&1 || {
	echo "FAIL: follower process not running (pid $follower_pid)" >&2
	exit 1
}

echo "leader=$leader_port isolating_follower=$follower_id:$follower_port pid=$follower_pid"

baseline=$(wait_counts_match_all) || {
	echo "FAIL: cluster not converged before isolation" >&2
	exit 1
}
echo "baseline converged count=$baseline"

echo "Temporarily isolating follower $follower_id via SIGSTOP for ${ISOLATION_SECONDS}s..."
kill -STOP "$follower_pid"
sleep "$ISOLATION_SECONDS"

echo "Submitting writes while follower is isolated..."
submit_payment "$leader_port" "isolation-a-$(date +%s)"
submit_payment "$leader_port" "isolation-b-$(date +%s)"
submit_payment "$leader_port" "isolation-c-$(date +%s)"

echo "Resuming follower $follower_id via SIGCONT..."
kill -CONT "$follower_pid"

final_count=$(wait_counts_match_all) || {
	echo "FAIL: cluster did not reconverge after follower resume" >&2
	exit 1
}

final_status=$(status_for_port "$follower_port")
final_role=$(extract_string "$final_status" "role")
if [ "$final_role" != "FOLLOWER" ] && [ "$final_role" != "LEADER" ]; then
	echo "FAIL: resumed node did not recover to active role (role=$final_role)" >&2
	exit 1
fi

echo "PASS: temporary isolation + convergence verified"
echo "- follower $follower_id resumed and rejoined with role=$final_role"
echo "- baseline_count=$baseline final_count=$final_count"
echo "- all three ledgers converged after recovery"
