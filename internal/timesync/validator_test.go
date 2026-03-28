package timesync

import (
    "testing"
    "time"
)

func TestValidator_ValidMessage(t *testing.T) {
    tracker := NewSkewTracker()
    tracker.Record("node-1", 100*time.Millisecond)
    v := NewValidator(tracker)

    // A message timestamped just now should be valid
    err := v.Validate("node-1", time.Now())
    if err != nil {
        t.Errorf("expected valid message, got error: %v", err)
    }
}

func TestValidator_TooOldMessage(t *testing.T) {
    tracker := NewSkewTracker()
    v := NewValidator(tracker)

    // A message from 5 seconds ago should be rejected
    oldTime := time.Now().Add(-5 * time.Second)
    err := v.Validate("node-1", oldTime)
    if err == nil {
        t.Errorf("expected error for old message, got nil")
    }
}

func TestValidator_TooFarInFuture(t *testing.T) {
    tracker := NewSkewTracker()
    v := NewValidator(tracker)

    // A message 2 seconds in the future should be rejected
    futureTime := time.Now().Add(2 * time.Second)
    err := v.Validate("node-1", futureTime)
    if err == nil {
        t.Errorf("expected error for future message, got nil")
    }
}

func TestValidator_SkewExceedsTolerance(t *testing.T) {
    tracker := NewSkewTracker()
    tracker.Record("node-2", 600*time.Millisecond) // exceeds 500ms tolerance
    v := NewValidator(tracker)

    err := v.Validate("node-2", time.Now())
    if err == nil {
        t.Errorf("expected error for node with excessive skew, got nil")
    }
}