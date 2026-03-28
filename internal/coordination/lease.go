package coordination

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-zookeeper/zk"
)

type leaderLease struct {
	LeaderID  string `json:"leader_id"`
	LeaderURL string `json:"leader_url"`
	Term      int64  `json:"term"`
	Since     string `json:"since,omitempty"`
	LeaseID   string `json:"lease_id,omitempty"`
}

func (m *Manager) tryAcquireLeaderLease() (leaderLease, bool, error) {
	currentLease, stat, err := m.readCurrentLeaderLease()
	switch {
	case err == nil:
		if stat != nil && stat.EphemeralOwner == m.conn.SessionID() && currentLease.LeaderID == m.cfg.NodeID {
			return currentLease, false, nil
		}
		return currentLease, false, nil
	case !errors.Is(err, zk.ErrNoNode):
		return leaderLease{}, false, err
	}

	lease := leaderLease{
		LeaderID:  m.cfg.NodeID,
		LeaderURL: m.cfg.BaseURL,
		Term:      0,
		Since:     time.Now().UTC().Format(time.RFC3339),
		LeaseID:   fmt.Sprintf("%s-%d", m.cfg.NodeID, time.Now().UTC().UnixNano()),
	}

	payload, err := json.Marshal(lease)
	if err != nil {
		return leaderLease{}, false, err
	}

	_, err = m.conn.Create(m.leaderPath(), payload, zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
	if err != nil {
		if errors.Is(err, zk.ErrNodeExists) {
			existingLease, _, readErr := m.readCurrentLeaderLease()
			return existingLease, false, readErr
		}
		return leaderLease{}, false, err
	}

	term, err := m.bumpTermWithCAS()
	if err != nil {
		m.releaseLeaderLeaseIfOwned()
		return leaderLease{}, false, err
	}

	lease.Term = term
	payload, err = json.Marshal(lease)
	if err != nil {
		m.releaseLeaderLeaseIfOwned()
		return leaderLease{}, false, err
	}

	_, err = m.conn.Set(m.leaderPath(), payload, -1)
	if err != nil {
		m.releaseLeaderLeaseIfOwned()
		return leaderLease{}, false, err
	}

	m.recordElectionWon()
	return lease, true, nil
}

func (m *Manager) readCurrentLeaderLease() (leaderLease, *zk.Stat, error) {
	data, stat, err := m.conn.Get(m.leaderPath())
	if err != nil {
		return leaderLease{}, nil, err
	}

	var lease leaderLease
	if err := json.Unmarshal(data, &lease); err != nil {
		return leaderLease{}, nil, err
	}
	return lease, stat, nil
}

func (m *Manager) verifyLeadershipStillHeld() (bool, error) {
	data, stat, err := m.conn.Get(m.leaderPath())
	if err != nil {
		if errors.Is(err, zk.ErrNoNode) {
			return false, nil
		}
		return false, err
	}
	if stat == nil || stat.EphemeralOwner != m.conn.SessionID() {
		return false, nil
	}

	var lease leaderLease
	if err := json.Unmarshal(data, &lease); err != nil {
		return false, err
	}
	if lease.LeaderID != m.cfg.NodeID {
		return false, nil
	}

	return true, nil
}

func (m *Manager) refreshStatusFromLease() error {
	lease, _, err := m.readCurrentLeaderLease()
	if err != nil {
		return err
	}

	logHead, err := m.readLogHead()
	if err != nil {
		return err
	}

	held, err := m.verifyLeadershipStillHeld()
	if err != nil {
		return err
	}

	role := RoleFollower
	if held {
		role = RoleLeader
	}

	m.mu.Lock()
	prevRole := m.status.Role
	prevLeaderID := m.status.LeaderID
	prevTerm := m.status.Term
	m.status.Role = role
	m.status.LeaderID = lease.LeaderID
	m.status.LeaderURL = lease.LeaderURL
	m.status.Term = lease.Term
	m.status.LogHead = logHead
	m.status.ZKError = ""
	m.lastKnownLease = lease
	m.mu.Unlock()

	if prevRole != role || prevLeaderID != lease.LeaderID || prevTerm != lease.Term {
		m.cfg.Logger.Printf(
			"leadership status update node_id=%s role=%s leader_id=%s term=%d lease_id=%s (prev_role=%s prev_leader_id=%s prev_term=%d)",
			m.cfg.NodeID,
			role,
			lease.LeaderID,
			lease.Term,
			lease.LeaseID,
			prevRole,
			prevLeaderID,
			prevTerm,
		)
	}

	return nil
}

func (m *Manager) releaseLeaderLeaseIfOwned() {
	held, err := m.verifyLeadershipStillHeld()
	if err != nil || !held {
		return
	}
	_ = m.conn.Delete(m.leaderPath(), -1)
}
