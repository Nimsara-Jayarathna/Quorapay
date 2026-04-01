package coordination

import (
	"errors"
	"sort"
	"strings"

	"github.com/go-zookeeper/zk"
)

func (m *Manager) ensureMemberNode() error {
	memberPath := m.memberPath()
	exists, _, err := m.conn.Exists(memberPath)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	_, err = m.conn.Create(memberPath, []byte(m.cfg.BaseURL), zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
	if err != nil && !errors.Is(err, zk.ErrNodeExists) {
		return err
	}
	return nil
}

func (m *Manager) ensureCandidateNode() error {
	m.mu.RLock()
	candidatePath := m.candidatePath
	m.mu.RUnlock()

	if candidatePath != "" {
		exists, _, err := m.conn.Exists(candidatePath)
		if err == nil && exists {
			return nil
		}
	}

	createdPath, err := m.conn.Create(m.electionPath()+"/candidate-", []byte(m.cfg.NodeID), zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.candidatePath = createdPath
	m.mu.Unlock()

	return nil
}

func (m *Manager) determineLeaderEligible() (bool, error) {
	children, _, err := m.conn.Children(m.electionPath())
	if err != nil {
		return false, err
	}

	candidates := make([]string, 0, len(children))
	for _, child := range children {
		if strings.HasPrefix(child, "candidate-") {
			candidates = append(candidates, child)
		}
	}
	if len(candidates) == 0 {
		m.clearEligibility()
		return false, nil
	}

	sort.Strings(candidates)

	m.mu.RLock()
	selfCandidate := m.candidatePath
	m.mu.RUnlock()

	eligible := selfCandidate != "" && strings.TrimPrefix(selfCandidate, m.electionPath()+"/") == candidates[0]
	if eligible {
		m.markEligible()
	} else {
		m.clearEligibility()
	}
	return eligible, nil
}
