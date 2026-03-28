#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-90}"
POLL_SECONDS="${POLL_SECONDS:-1}"

port_for_node() {
	case "$1" in
		A) echo "8001" ;;
		B) echo "8002" ;;
		C) echo "8003" ;;
		*) echo "" ;;
	esac
}

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

echo "Checking initial single-leader condition..."
initial=$(wait_for_single_leader) || {
	echo "FAIL: could not establish initial single leader within timeout" >&2
	exit 1
}

initial_leader=$(echo "$initial" | awk '{print $1}')
initial_term=$(echo "$initial" | awk '{print $2}')
echo "Initial leader=$initial_leader term=$initial_term"

echo "Triggering leader failover..."
"$ROOT_DIR/scripts/kill-leader.sh"

echo "Waiting for new single leader..."
post_failover=$(wait_for_single_leader) || {
	echo "FAIL: no stable single leader after failover within timeout" >&2
	exit 1
}

new_leader=$(echo "$post_failover" | awk '{print $1}')
new_term=$(echo "$post_failover" | awk '{print $2}')
echo "After failover leader=$new_leader term=$new_term"

if [ "$new_leader" = "$initial_leader" ]; then
	echo "FAIL: leader did not change after failover" >&2
	exit 1
fi

if [ "$new_term" -le "$initial_term" ]; then
	echo "FAIL: term did not increase after failover (before=$initial_term after=$new_term)" >&2
	exit 1
fi

echo "Restarting old leader node $initial_leader for stale-leader fencing check..."
"$ROOT_DIR/scripts/run-node.sh" "$initial_leader"

old_port=$(port_for_node "$initial_leader")
if [ -z "$old_port" ]; then
	echo "FAIL: unknown old leader node id '$initial_leader'" >&2
	exit 1
fi

echo "Waiting for restarted node $initial_leader to report FOLLOWER..."
start_ts=$(date +%s)
while :; do
	now_ts=$(date +%s)
	if [ $((now_ts - start_ts)) -gt "$TIMEOUT_SECONDS" ]; then
		echo "FAIL: restarted node did not reach expected follower state in time" >&2
		exit 1
	fi

	json=$(status_for_port "$old_port")
	if [ -n "$json" ]; then
		role=$(extract_string "$json" "role")
		leader_id=$(extract_string "$json" "leader_id")
		if [ "$role" = "FOLLOWER" ] && [ "$leader_id" = "$new_leader" ]; then
			echo "Rejoined node $initial_leader is FOLLOWER and recognizes leader $new_leader"
			break
		fi
	fi
	sleep "$POLL_SECONDS"
done

echo "Re-checking single-leader invariant after restart..."
final=$(wait_for_single_leader) || {
	echo "FAIL: single-leader invariant not stable after restart" >&2
	exit 1
}
final_leader=$(echo "$final" | awk '{print $1}')
final_term=$(echo "$final" | awk '{print $2}')

if [ "$final_leader" != "$new_leader" ]; then
	echo "FAIL: leader changed unexpectedly after old-leader restart (expected $new_leader got $final_leader)" >&2
	exit 1
fi

echo "PASS:"
echo "- exactly one leader before and after failover"
echo "- term increased on leadership change ($initial_term -> $new_term)"
echo "- restarted old leader was fenced and rejoined as follower"
echo "- final leader remained $final_leader at term $final_term"
