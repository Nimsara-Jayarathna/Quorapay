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

// MaxAbsOffset returns the maximum absolute offset observed across tracked nodes.
func (s *SkewTracker) MaxAbsOffset() (time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.offsets) == 0 {
		return 0, false
	}
	var max time.Duration
	first := true
	for _, offset := range s.offsets {
		if offset < 0 {
			offset = -offset
		}
		if first || offset > max {
			max = offset
			first = false
		}
	}
	return max, true
}
