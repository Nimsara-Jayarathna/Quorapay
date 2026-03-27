#!/bin/sh

set -eu

PORT="${1:-8003}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-90}"
POLL_SECONDS="${POLL_SECONDS:-1}"

extract_field() {
	echo "$1" | sed -n "s/.*\"$2\":\"\\([^\"]*\\)\".*/\\1/p"
}

status_json() {
	curl -fsS "http://localhost:$PORT/status" 2>/dev/null || true
}

wait_for_initial_healthy() {
	start_ts=$(date +%s)
	while :; do
		now_ts=$(date +%s)
		if [ $((now_ts - start_ts)) -gt "$TIMEOUT_SECONDS" ]; then
			echo "timeout waiting for initial HEALTHY state on port $PORT" >&2
			exit 1
		fi

		json=$(status_json)
		if [ -n "$json" ]; then
			state=$(extract_field "$json" "fault_state")
			if [ "$state" = "HEALTHY" ]; then
				echo "initial state is HEALTHY on port $PORT"
				return
			fi
		fi
		sleep "$POLL_SECONDS"
	done
}

echo "Starting fault-state sequence check on http://localhost:$PORT/status"
wait_for_initial_healthy
echo "Now stop ZooKeeper, wait 2-3 seconds, then start ZooKeeper again."
echo "Expected sequence: FAILED -> RECOVERING -> REJOINED -> HEALTHY"

expected_failed=0
expected_recovering=0
expected_rejoined=0
expected_healthy=0
start_ts=$(date +%s)

while :; do
	now_ts=$(date +%s)
	if [ $((now_ts - start_ts)) -gt "$TIMEOUT_SECONDS" ]; then
		echo "timeout before sequence completed on port $PORT" >&2
		exit 1
	fi

	json=$(status_json)
	if [ -z "$json" ]; then
		sleep "$POLL_SECONDS"
		continue
	fi

	state=$(extract_field "$json" "fault_state")
	reason=$(extract_field "$json" "last_fault_reason")
	ts=$(extract_field "$json" "timestamp")
	if [ -n "$state" ]; then
		echo "$ts state=$state reason=$reason"
	fi

	if [ "$expected_failed" -eq 0 ]; then
		if [ "$state" = "FAILED" ]; then
			expected_failed=1
		fi
		sleep "$POLL_SECONDS"
		continue
	fi

	if [ "$expected_recovering" -eq 0 ]; then
		if [ "$state" = "RECOVERING" ]; then
			expected_recovering=1
		fi
		sleep "$POLL_SECONDS"
		continue
	fi

	if [ "$expected_rejoined" -eq 0 ]; then
		if [ "$state" = "REJOINED" ]; then
			expected_rejoined=1
		fi
		sleep "$POLL_SECONDS"
		continue
	fi

	if [ "$expected_healthy" -eq 0 ]; then
		if [ "$state" = "HEALTHY" ]; then
			expected_healthy=1
			break
		fi
		sleep "$POLL_SECONDS"
		continue
	fi
done

echo "PASS: observed sequence FAILED -> RECOVERING -> REJOINED -> HEALTHY on node port $PORT"
