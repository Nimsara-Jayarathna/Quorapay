#!/bin/sh

# Shared cluster topology resolution for local scripts.
# Priority:
# 1) NODES (explicit override)
# 2) CLUSTER_SIZE (odd number >= 3)

resolve_nodes_from_size() {
	size="${CLUSTER_SIZE:-3}"
	base_port="${CLUSTER_BASE_PORT:-8001}"

	case "$size" in
		''|*[!0-9]*)
			echo "invalid CLUSTER_SIZE='$size' (must be an odd integer >= 3)" >&2
			return 1
			;;
	esac
	case "$base_port" in
		''|*[!0-9]*)
			echo "invalid CLUSTER_BASE_PORT='$base_port' (must be a positive integer)" >&2
			return 1
			;;
	esac
	if [ "$size" -lt 3 ] || [ $((size % 2)) -eq 0 ]; then
		echo "invalid CLUSTER_SIZE='$size' (must be odd and >= 3)" >&2
		return 1
	fi
	if [ "$size" -gt 26 ]; then
		echo "invalid CLUSTER_SIZE='$size' (max supported is 26: A..Z)" >&2
		return 1
	fi

	awk -v n="$size" -v start="$base_port" 'BEGIN {
		out = ""
		for (i = 0; i < n; i++) {
			node = sprintf("%c", 65 + i)
			port = start + i
			entry = node ":" port
			if (i == 0) out = entry
			else out = out "," entry
		}
		print out
	}'
}

if [ -z "${NODES:-}" ]; then
	NODES="$(resolve_nodes_from_size)"
fi
