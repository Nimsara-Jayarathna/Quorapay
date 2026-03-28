#!/bin/sh

set -eu

TOTAL_REQUESTS="${1:-120}"
CONCURRENCY="${2:-12}"
TARGET_MODE="${3:-leader}" # leader|round_robin
AMOUNT="${AMOUNT:-10}"
CURRENCY="${CURRENCY:-USD}"
MAX_LATENCY_MS_WARN="${MAX_LATENCY_MS_WARN:-1000}"
MAX_FAILURES="${MAX_FAILURES:-0}"
RETRY_ATTEMPTS="${RETRY_ATTEMPTS:-2}"
RETRY_SLEEP_SECONDS="${RETRY_SLEEP_SECONDS:-0.2}"
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)

. "$ROOT_DIR/scripts/lib/cluster_env.sh"

if [ "$TOTAL_REQUESTS" -le 0 ] || [ "$CONCURRENCY" -le 0 ]; then
	echo "usage: ./scripts/tests/test-high-rate-throughput.sh [total_requests] [concurrency] [leader|round_robin]" >&2
	exit 1
fi

extract_string() {
	echo "$1" | sed -n "s/.*\"$2\":\"\\([^\"]*\\)\".*/\\1/p"
}

status_for_port() {
	curl -fsS "http://localhost:$1/status" 2>/dev/null || true
}

find_leader_port() {
	for spec in $(echo "$NODES" | tr ',' ' '); do
		port=$(echo "$spec" | cut -d: -f2)
		json=$(status_for_port "$port")
		[ -n "$json" ] || continue
		role=$(extract_string "$json" "role")
		if [ "$role" = "LEADER" ]; then
			echo "$port"
			return 0
		fi
	done
	return 1
}

pick_port_for_index() {
	idx="$1"
	if [ "$TARGET_MODE" = "leader" ]; then
		find_leader_port
		return
	fi
	pos=$(( (idx - 1) % $(echo "$NODES" | tr ',' ' ' | wc -w | tr -d ' ') + 1 ))
	cur=1
	for spec in $(echo "$NODES" | tr ',' ' '); do
		port=$(echo "$spec" | cut -d: -f2)
		if [ "$cur" -eq "$pos" ]; then
			echo "$port"
			return 0
		fi
		cur=$((cur + 1))
	done
	return 1
}

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

START_NS=$(date +%s%N)
i=1
while [ "$i" -le "$TOTAL_REQUESTS" ]; do
	(
		port=$(pick_port_for_index "$i")
		payment_id="perf-$i-$(date +%s%N)"
		req_start=$(date +%s%N)
		attempt=0
		code="000"
		while [ "$attempt" -le "$RETRY_ATTEMPTS" ]; do
			code=$(curl -sS -o "$TMP_DIR/body-$i.json" -w "%{http_code}" \
				-X POST "http://localhost:$port/pay" \
				-H "Content-Type: application/json" \
				-d "{\"payment_id\":\"$payment_id\",\"amount\":$AMOUNT,\"currency\":\"$CURRENCY\"}" || echo "000")
			if [ "$code" = "200" ]; then
				break
			fi
			# Retry transient write-path failures.
			if [ "$code" != "503" ] && [ "$code" != "000" ]; then
				break
			fi
			attempt=$((attempt + 1))
			if [ "$attempt" -le "$RETRY_ATTEMPTS" ]; then
				sleep "$RETRY_SLEEP_SECONDS"
			fi
		done
		req_end=$(date +%s%N)
		lat_ms=$(( (req_end - req_start) / 1000000 ))

		if [ "$code" = "200" ]; then
			echo "OK $lat_ms" >"$TMP_DIR/result-$i.txt"
		else
			msg=$(sed -n 's/.*"message":"\([^"]*\)".*/\1/p' "$TMP_DIR/body-$i.json" | head -n 1)
			echo "ERR $lat_ms $code ${msg:-unknown} attempt=$attempt" >"$TMP_DIR/result-$i.txt"
		fi
	) &

	if [ $((i % CONCURRENCY)) -eq 0 ]; then
		wait
	fi
	i=$((i + 1))
done
wait
END_NS=$(date +%s%N)

duration_ns=$((END_NS - START_NS))
duration_ms=$((duration_ns / 1000000))
if [ "$duration_ms" -le 0 ]; then
	duration_ms=1
fi

ok_count=0
err_count=0
sum_latency=0
max_latency=0

for f in "$TMP_DIR"/result-*.txt; do
	[ -f "$f" ] || continue
	status=$(awk '{print $1}' "$f")
	lat=$(awk '{print $2}' "$f")
	sum_latency=$((sum_latency + lat))
	if [ "$lat" -gt "$max_latency" ]; then
		max_latency="$lat"
	fi
	if [ "$status" = "OK" ]; then
		ok_count=$((ok_count + 1))
	else
		err_count=$((err_count + 1))
	fi
done

if [ "$TOTAL_REQUESTS" -gt 0 ]; then
	avg_latency=$((sum_latency / TOTAL_REQUESTS))
else
	avg_latency=0
fi

rps=$(( (ok_count * 1000) / duration_ms ))

if ls "$TMP_DIR"/result-*.txt >/dev/null 2>&1; then
	p95=$(awk '{print $2}' "$TMP_DIR"/result-*.txt | sort -n | awk -v n="$TOTAL_REQUESTS" '
		BEGIN {
			target = int((n * 95 + 99) / 100);
			if (target < 1) target = 1;
		}
		NR == target { print $1; found=1; exit }
		END { if (!found) print 0 }
	')
else
	p95=0
fi

echo "Performance Summary:"
echo "- total_requests=$TOTAL_REQUESTS concurrency=$CONCURRENCY target_mode=$TARGET_MODE"
echo "- success=$ok_count failures=$err_count"
echo "- duration_ms=$duration_ms throughput_rps=$rps"
echo "- latency_avg_ms=$avg_latency latency_p95_ms=$p95 latency_max_ms=$max_latency"

if [ "$err_count" -gt 0 ]; then
	echo "- error_code_breakdown:"
	awk '/^ERR /{print $3}' "$TMP_DIR"/result-*.txt | sort | uniq -c | awk '{printf "  - code=%s count=%s\n", $2, $1}'
fi

if [ "$err_count" -gt "$MAX_FAILURES" ]; then
	echo "FAIL: observed failed requests under load (failures=$err_count max_failures=$MAX_FAILURES)" >&2
	exit 1
fi

if [ "$max_latency" -gt "$MAX_LATENCY_MS_WARN" ]; then
	echo "WARN: max latency ${max_latency}ms exceeded warning threshold ${MAX_LATENCY_MS_WARN}ms"
fi

echo "PASS: high-rate throughput run completed within allowed failure threshold (failures=$err_count max_failures=$MAX_FAILURES)"
