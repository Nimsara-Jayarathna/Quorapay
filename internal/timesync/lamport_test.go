package timesync

import "testing"

func TestLamportClock_TickIncrementsSequentially(t *testing.T) {
	c := &LamportClock{}

	if got := c.Tick(); got != 1 {
		t.Fatalf("Tick #1 = %d, want 1", got)
	}
	if got := c.Tick(); got != 2 {
		t.Fatalf("Tick #2 = %d, want 2", got)
	}
	if got := c.Tick(); got != 3 {
		t.Fatalf("Tick #3 = %d, want 3", got)
	}
}

func TestLamportClock_ReceiveHigherTimestamp(t *testing.T) {
	c := &LamportClock{}

	// local: 0, receive ts=10 => max(0,10)+1 = 11
	if got := c.Receive(10); got != 11 {
		t.Fatalf("Receive(10) = %d, want 11", got)
	}
	// now local should be 11
	if now := c.Now(); now != 11 {
		t.Fatalf("Now() after Receive(10) = %d, want 11", now)
	}
}

func TestLamportClock_ReceiveLowerTimestamp(t *testing.T) {
	c := &LamportClock{}

	// move local clock forward: local becomes 3
	c.Tick()
	c.Tick()
	c.Tick()

	// local: 3, receive ts=2 => max(3,2)+1 = 4
	if got := c.Receive(2); got != 4 {
		t.Fatalf("Receive(2) = %d, want 4", got)
	}
	// now local should be 4
	if now := c.Now(); now != 4 {
		t.Fatalf("Now() after Receive(2) = %d, want 4", now)
	}
}