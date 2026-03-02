package coordination

import (
	"time"
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
	Timestamp       string `json:"timestamp"`
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
