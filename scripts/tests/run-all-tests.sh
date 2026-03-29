#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
TEST_DIR="$ROOT_DIR/scripts/tests"
. "$ROOT_DIR/scripts/lib/cluster_env.sh"

# Defaults
CYCLES="${CYCLES:-3}"
ISOLATION_SECONDS="${ISOLATION_SECONDS:-6}"
LOAD_REQUESTS="${LOAD_REQUESTS:-120}"
LOAD_CONCURRENCY="${LOAD_CONCURRENCY:-12}"
LOAD_MODE="${LOAD_MODE:-leader}"
INCLUDE_MANUAL="${INCLUDE_MANUAL:-0}"
FAULT_STATE_PORT="${FAULT_STATE_PORT:-}"

run_test() {
	name="$1"
	shift
	echo ""
	echo "==> RUN: $name"
	"$@"
	echo "==> PASS: $name"
}

wait_all_nodes_up() {
	timeout="${1:-60}"
	elapsed=0
	while [ "$elapsed" -lt "$timeout" ]; do
		all_up=1
		for spec in $(echo "$NODES" | tr ',' ' '); do
			id=$(echo "$spec" | cut -d: -f1)
			port=$(echo "$spec" | cut -d: -f2)
			if ! curl -fsS "http://localhost:$port/status" >/dev/null 2>&1; then
				all_up=0
				break
			fi
		done
		if [ "$all_up" -eq 1 ]; then
			return 0
		fi
		sleep 1
		elapsed=$((elapsed + 1))
	done
	return 1
}

echo "Running Quorapay test suite"
echo "Config:"
echo "- CLUSTER_SIZE=${CLUSTER_SIZE:-3}"
echo "- CYCLES=$CYCLES"
echo "- ISOLATION_SECONDS=$ISOLATION_SECONDS"
echo "- LOAD_REQUESTS=$LOAD_REQUESTS"
echo "- LOAD_CONCURRENCY=$LOAD_CONCURRENCY"
echo "- LOAD_MODE=$LOAD_MODE"
echo "- INCLUDE_MANUAL=$INCLUDE_MANUAL"
echo ""
echo "Prerequisites:"
echo "1) ZooKeeper is up"
echo "2) Nodes are running (./scripts/run-nodes.sh)"
echo ""

if ! wait_all_nodes_up 90; then
	echo "FAIL: not all configured nodes are reachable before tests start." >&2
	echo "Configured NODES=$NODES" >&2
	echo "Check with: for p in $(echo \"$NODES\" | tr ',' ' ' | cut -d: -f2); do curl -sS http://localhost:$p/status; done" >&2
	exit 1
fi

run_test "Election Consensus" "$TEST_DIR/test-election-consensus.sh"
run_test "Follower Rejoin Convergence" "$TEST_DIR/test-follower-rejoin-convergence.sh"
run_test "Repeated Restart No Divergence" "$TEST_DIR/test-repeated-restart-no-divergence.sh" "$CYCLES"
run_test "Temporary Node Isolation Convergence" "$TEST_DIR/test-temporary-node-isolation-convergence.sh" "$ISOLATION_SECONDS"
run_test "High Rate Throughput" "$TEST_DIR/test-high-rate-throughput.sh" "$LOAD_REQUESTS" "$LOAD_CONCURRENCY" "$LOAD_MODE"

if [ "$INCLUDE_MANUAL" = "1" ]; then
	echo ""
	echo "==> MANUAL TESTS ENABLED"
	run_test "ZooKeeper Outage Recovery (manual prompt inside test)" "$TEST_DIR/test-zk-outage-recovery.sh"

	if [ -n "$FAULT_STATE_PORT" ]; then
		run_test "Fault State Sequence (manual prompt inside test)" "$TEST_DIR/test-fault-state-sequence.sh" "$FAULT_STATE_PORT"
	else
		echo "Skipping fault-state sequence: set FAULT_STATE_PORT (example: FAULT_STATE_PORT=8003)."
	fi
fi

echo ""
echo "All requested tests completed."
