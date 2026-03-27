#!/bin/sh

set -eu

TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-120}"
POLL_SECONDS="${POLL_SECONDS:-1}"

extract_string() {
	echo "$1" | sed -n "s/.*\"$2\":\"\\([^\"]*\\)\".*/\\1/p"
}

extract_number() {
	echo "$1" | sed -n "s/.*\"$2\":\\([-0-9][0-9]*\\).*/\\1/p"
}

status_for_port() {
	curl -fsS "http://localhost:$1/status" 2>/dev/null || true
}

find_single_leader() {
	leader_count=0
	leader_id=""
	leader_term=""

	for port in 8001 8002 8003; do
		json=$(status_for_port "$port")
		if [ -z "$json" ]; then
			continue
		fi
		role=$(extract_string "$json" "role")
		if [ "$role" = "LEADER" ]; then
			leader_count=$((leader_count + 1))
			leader_id=$(extract_string "$json" "node_id")
			leader_term=$(extract_number "$json" "term")
		fi
	done

	if [ "$leader_count" -eq 1 ] && [ -n "$leader_id" ] && [ -n "$leader_term" ]; then
		echo "$leader_id $leader_term"
		return 0
	fi
	return 1
}

wait_for_single_leader() {
	start_ts=$(date +%s)
	while :; do
		now_ts=$(date +%s)
		if [ $((now_ts - start_ts)) -gt "$TIMEOUT_SECONDS" ]; then
			return 1
		fi
		if result=$(find_single_leader); then
			echo "$result"
			return 0
		fi
		sleep "$POLL_SECONDS"
	done
}

all_nodes_unknown_with_zk_error() {
	for port in 8001 8002 8003; do
		json=$(status_for_port "$port")
		if [ -z "$json" ]; then
			return 1
		fi

		role=$(extract_string "$json" "role")
		err_msg=$(extract_string "$json" "zk_error")
		if [ "$role" != "UNKNOWN" ] || [ -z "$err_msg" ]; then
			return 1
		fi
	done
	return 0
}

wait_for_all_unknown_with_error() {
	start_ts=$(date +%s)
	while :; do
		now_ts=$(date +%s)
		if [ $((now_ts - start_ts)) -gt "$TIMEOUT_SECONDS" ]; then
			return 1
		fi
		if all_nodes_unknown_with_zk_error; then
			return 0
		fi
		sleep "$POLL_SECONDS"
	done
}

echo "Checking pre-outage single-leader state..."
before=$(wait_for_single_leader) || {
	echo "FAIL: could not find single leader before outage" >&2
	exit 1
}
before_leader=$(echo "$before" | awk '{print $1}')
before_term=$(echo "$before" | awk '{print $2}')
echo "Pre-outage leader=$before_leader term=$before_term"

echo "Now stop ZooKeeper service, then press ENTER."
read -r _

echo "Waiting for all nodes to report role=UNKNOWN with zk_error..."
wait_for_all_unknown_with_error || {
	echo "FAIL: nodes did not converge to UNKNOWN + zk_error during outage" >&2
	exit 1
}
echo "Outage behavior verified: all nodes UNKNOWN with zk_error"

echo "Now start ZooKeeper service again, then press ENTER."
read -r _

echo "Waiting for post-recovery single leader..."
after=$(wait_for_single_leader) || {
	echo "FAIL: single leader was not re-established after ZooKeeper recovery" >&2
	exit 1
}
after_leader=$(echo "$after" | awk '{print $1}')
after_term=$(echo "$after" | awk '{print $2}')
echo "Post-recovery leader=$after_leader term=$after_term"

if [ "$after_term" -lt "$before_term" ]; then
	echo "FAIL: term moved backwards ($before_term -> $after_term)" >&2
	exit 1
fi

echo "PASS:"
echo "- pre-outage cluster had exactly one leader"
echo "- during outage all nodes reported UNKNOWN with zk_error"
echo "- after recovery exactly one leader was restored"
echo "- term remained monotonic ($before_term -> $after_term)"
