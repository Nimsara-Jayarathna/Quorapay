package timesync

import "testing"

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