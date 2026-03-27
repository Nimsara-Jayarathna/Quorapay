package coordination

import (
	"fmt"
	"time"
)

const (
	FaultStateHealthy    = "HEALTHY"
	FaultStateFailed     = "FAILED"
	FaultStateRecovering = "RECOVERING"
	FaultStateRejoined   = "REJOINED"
)

type Status struct {
	NodeID          string `json:"node_id"`
	Role            string `json:"role"`
	LeaderID        string `json:"leader_id"`
	LeaderURL       string `json:"leader_url"`
	Term            int64  `json:"term"`
	LogHead         int64  `json:"log_head"`
	ElectionCount   int64  `json:"election_count,omitempty"`
	LastElectionMS  int64  `json:"last_election_ms,omitempty"`
	StatusRefreshMS int64  `json:"status_refresh_ms,omitempty"`
	ZKAddr          string `json:"zk_addr"`
	StoragePath     string `json:"storage_path"`
	ZKError         string `json:"zk_error,omitempty"`
	FaultState      string `json:"fault_state"`
	LastFaultReason string `json:"last_fault_reason,omitempty"`
	LastStateChange string `json:"last_state_change,omitempty"`
	Timestamp       string `json:"timestamp"`
}

func isValidFaultState(state string) bool {
	switch state {
	case FaultStateHealthy, FaultStateFailed, FaultStateRecovering, FaultStateRejoined:
		return true
	default:
		return false
	}
}

func canTransitionFaultState(current string, next string) bool {
	switch current {
	case "":
		return next == FaultStateRecovering
	case FaultStateRecovering:
		return next == FaultStateRejoined || next == FaultStateFailed
	case FaultStateRejoined:
		return next == FaultStateHealthy || next == FaultStateFailed
	case FaultStateHealthy:
		return next == FaultStateFailed || next == FaultStateRecovering
	case FaultStateFailed:
		return next == FaultStateRecovering
	default:
		return false
	}
}

func (m *Manager) setFaultState(next string, reason string) error {
	if !isValidFaultState(next) {
		return fmt.Errorf("invalid fault state: %s", next)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	m.mu.Lock()
	defer m.mu.Unlock()

	current := m.status.FaultState
	if current == next {
		if reason != "" {
			m.status.LastFaultReason = reason
		}
		return nil
	}

	if !canTransitionFaultState(current, next) {
		return fmt.Errorf("invalid fault state transition: %s -> %s", current, next)
	}

	m.status.FaultState = next
	m.status.LastStateChange = now
	if reason != "" {
		m.status.LastFaultReason = reason
	}
	return nil
}

func (m *Manager) markEligible() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.eligibleSince.IsZero() {
		m.eligibleSince = time.Now()
	}
}

func (m *Manager) clearEligibility() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eligibleSince = time.Time{}
}

func (m *Manager) recordElectionWon() {
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.ElectionCount++
	if !m.eligibleSince.IsZero() {
		m.status.LastElectionMS = now.Sub(m.eligibleSince).Milliseconds()
	}
	m.eligibleSince = time.Time{}
}

func (m *Manager) recordStatusRefreshTick() {
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.lastLoopStart.IsZero() {
		m.status.StatusRefreshMS = now.Sub(m.lastLoopStart).Milliseconds()
	}
	m.lastLoopStart = now
}

func (m *Manager) setNoLeaderStatus() {
	logHead := int64(0)
	if head, err := m.readLogHead(); err == nil {
		logHead = head
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.Role = RoleUnknown
	m.status.LeaderID = ""
	m.status.LeaderURL = ""
	m.status.Term = 0
	m.status.LogHead = logHead
}

func (m *Manager) setZKError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.ZKError = err.Error()
	m.status.Role = RoleUnknown
	m.status.LeaderID = ""
	m.status.LeaderURL = ""
	m.status.Term = 0
	m.status.LogHead = 0
}

func (m *Manager) clearZKError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.ZKError = ""
}
