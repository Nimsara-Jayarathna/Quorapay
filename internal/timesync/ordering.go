package timesync

import "strings"

// Transaction represents a timestamped event from a specific node.
type Transaction struct {
    Timestamp uint64
    NodeID    string
    Data      string
}

// Compare determines the order of two transactions.
// Returns:
//
//	-1 if a comes before b
//	 1 if b comes before a
//	 0 if they are identical (same timestamp and same node)
func Compare(a, b Transaction) int {
    if a.Timestamp < b.Timestamp {
        return -1
    }
    if a.Timestamp > b.Timestamp {
        return 1
    }
    // Timestamps are equal — break tie by NodeID alphabetically
    cmp := strings.Compare(a.NodeID, b.NodeID)
    if cmp < 0 {
        return -1
    }
    if cmp > 0 {
        return 1
    }
    return 0
}