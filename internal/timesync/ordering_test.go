package timesync

import (
	"sort"
	"testing"
)

func TestCompare_LowerTimestampFirst(t *testing.T) {
	a := Transaction{Timestamp: 1, NodeID: "node-1"}
	b := Transaction{Timestamp: 5, NodeID: "node-2"}

	if Compare(a, b) != -1 {
		t.Errorf("expected a to come before b, got %d", Compare(a, b))
	}
}

func TestCompare_HigherTimestampLast(t *testing.T) {
	a := Transaction{Timestamp: 10, NodeID: "node-1"}
	b := Transaction{Timestamp: 3, NodeID: "node-2"}

	if Compare(a, b) != 1 {
		t.Errorf("expected b to come before a, got %d", Compare(a, b))
	}
}

func TestCompare_TieBreakByNodeID(t *testing.T) {
	a := Transaction{Timestamp: 5, NodeID: "node-1"}
	b := Transaction{Timestamp: 5, NodeID: "node-2"}

	// "node-1" < "node-2" alphabetically, so a should come first
	if Compare(a, b) != -1 {
		t.Errorf("expected node-1 to come before node-2 on tie, got %d", Compare(a, b))
	}
}

func TestCompare_Identical(t *testing.T) {
	a := Transaction{Timestamp: 5, NodeID: "node-1"}
	b := Transaction{Timestamp: 5, NodeID: "node-1"}

	if Compare(a, b) != 0 {
		t.Errorf("expected identical transactions to return 0, got %d", Compare(a, b))
	}
}

func TestDeterministicOrdering_CloseTimestampsLogicalThenNodeID(t *testing.T) {
	items := []Transaction{
		// close logical times, with one exact tie on timestamp=101
		{Timestamp: 101, NodeID: "node-c", Data: "evt-c"},
		{Timestamp: 100, NodeID: "node-b", Data: "evt-b"},
		{Timestamp: 101, NodeID: "node-a", Data: "evt-a"},
	}

	sort.Slice(items, func(i, j int) bool {
		return Compare(items[i], items[j]) < 0
	})

	// Deterministic result:
	// 1) lower logical timestamp first (100)
	// 2) tie on logical timestamp resolved by node id (node-a before node-c)
	if items[0].NodeID != "node-b" || items[1].NodeID != "node-a" || items[2].NodeID != "node-c" {
		t.Fatalf("unexpected order: got [%s,%s,%s], want [node-b,node-a,node-c]", items[0].NodeID, items[1].NodeID, items[2].NodeID)
	}
}
