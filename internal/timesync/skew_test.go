package timesync

import (
    "testing"
    "time"
)

func TestSkewTracker_WithinTolerance(t *testing.T) {
    tracker := NewSkewTracker()
    tracker.Record("node-1", 200*time.Millisecond)

    ok, err := tracker.IsWithinTolerance("node-1")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !ok {
        t.Errorf("expected node-1 to be within tolerance")
    }
}

func TestSkewTracker_ExceedsTolerance(t *testing.T) {
    tracker := NewSkewTracker()
    tracker.Record("node-2", 600*time.Millisecond)

    ok, err := tracker.IsWithinTolerance("node-2")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if ok {
        t.Errorf("expected node-2 to exceed tolerance")
    }
}

func TestSkewTracker_NegativeOffset(t *testing.T) {
    tracker := NewSkewTracker()
    tracker.Record("node-3", -600*time.Millisecond)

    ok, err := tracker.IsWithinTolerance("node-3")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if ok {
        t.Errorf("expected negative large offset to exceed tolerance")
    }
}

func TestSkewTracker_UnknownNode(t *testing.T) {
    tracker := NewSkewTracker()

    _, err := tracker.IsWithinTolerance("node-unknown")
    if err == nil {
        t.Errorf("expected error for unknown node, got nil")
    }
}