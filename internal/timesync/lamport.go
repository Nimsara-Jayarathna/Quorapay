package timesync

import "sync"

// LamportClock is a thread-safe Lamport logical clock.
type LamportClock struct {
	mu   sync.Mutex
	time uint64
}

func NewLamportClock() *LamportClock {
	return &LamportClock{time: 0}
}

// Tick increments the clock for a local event and returns the new value.
func (c *LamportClock) Tick() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.time++
	return c.time
}

// Send increments the clock and returns the timestamp to attach to an outgoing message.
func (c *LamportClock) Send() uint64 { return c.Tick() }

// Receive updates the clock when receiving a message with timestamp ts.
// Rule: time = max(local, ts) + 1
func (c *LamportClock) Receive(ts uint64) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ts > c.time {
		c.time = ts
	}
	c.time++
	return c.time
}

// Now returns the current clock value without incrementing.
func (c *LamportClock) Now() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.time
}