package timesync

import (
    "fmt"
    "sync"
    "time"
)

// MaxAllowedSkew is the maximum tolerated clock difference between nodes.
const MaxAllowedSkew = 500 * time.Millisecond

// SkewTracker tracks observed clock differences from peer nodes.
type SkewTracker struct {
    mu      sync.Mutex
    offsets map[string]time.Duration // nodeID -> observed skew
}

// NewSkewTracker creates a new SkewTracker.
func NewSkewTracker() *SkewTracker {
    return &SkewTracker{
        offsets: make(map[string]time.Duration),
    }
}

// Record stores the observed clock offset for a peer node.
// offset = local time - peer's reported time
func (s *SkewTracker) Record(nodeID string, offset time.Duration) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.offsets[nodeID] = offset
}

// IsWithinTolerance returns true if the offset for a node is within MaxAllowedSkew.
func (s *SkewTracker) IsWithinTolerance(nodeID string) (bool, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    offset, ok := s.offsets[nodeID]
    if !ok {
        return false, fmt.Errorf("no skew data for node %s", nodeID)
    }
    if offset < 0 {
        offset = -offset
    }
    return offset <= MaxAllowedSkew, nil
}

// Get returns the recorded offset for a node.
func (s *SkewTracker) Get(nodeID string) (time.Duration, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    offset, ok := s.offsets[nodeID]
    return offset, ok
}