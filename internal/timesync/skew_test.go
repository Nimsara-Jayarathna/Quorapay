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

func TestSkewTracker_MaxAbsOffset(t *testing.T) {
	tracker := NewSkewTracker()
	if _, ok := tracker.MaxAbsOffset(); ok {
		t.Fatalf("expected ok=false for empty tracker")
	}

	tracker.Record("node-1", 120*time.Millisecond)
	tracker.Record("node-2", -450*time.Millisecond)
	tracker.Record("node-3", 300*time.Millisecond)

	max, ok := tracker.MaxAbsOffset()
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if max != 450*time.Millisecond {
		t.Fatalf("max = %v, want %v", max, 450*time.Millisecond)
	}
}
