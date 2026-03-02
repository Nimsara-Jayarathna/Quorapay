package coordination

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-zookeeper/zk"
)

func (m *Manager) ensureRegistered() error {
	if err := m.ensurePersistentPath(m.cfg.ZKRoot); err != nil {
		return err
	}
	if err := m.ensurePersistentPath(m.membersPath()); err != nil {
		return err
	}
	if err := m.ensurePersistentPath(m.electionPath()); err != nil {
		return err
	}
	if err := m.ensureTermNodeExists(); err != nil {
		return err
	}
	if err := m.ensureLogHeadExists(); err != nil {
		return err
	}
	if err := m.ensureMemberNode(); err != nil {
		return err
	}
	if err := m.ensureCandidateNode(); err != nil {
		return err
	}
	return nil
}

func (m *Manager) ensurePersistentPath(path string) error {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return nil
	}

	current := ""
	for _, part := range parts {
		current += "/" + part
		exists, _, err := m.conn.Exists(current)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		_, err = m.conn.Create(current, nil, 0, zk.WorldACL(zk.PermAll))
		if err != nil && !errors.Is(err, zk.ErrNodeExists) {
			return err
		}
	}

	return nil
}

func (m *Manager) ensureTermNodeExists() error {
	return m.ensureCounterNode(m.termPath())
}

func (m *Manager) ensureLogHeadExists() error {
	return m.ensureCounterNode(m.logHeadPath())
}

func (m *Manager) ensureCounterNode(path string) error {
	exists, _, err := m.conn.Exists(path)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	_, err = m.conn.Create(path, []byte("0"), 0, zk.WorldACL(zk.PermAll))
	if err != nil && !errors.Is(err, zk.ErrNodeExists) {
		return err
	}
	return nil
}

func (m *Manager) bumpTermWithCAS() (int64, error) {
	for {
		data, stat, err := m.conn.Get(m.termPath())
		if err != nil {
			return 0, err
		}

		currentTerm, err := parseCounter(data)
		if err != nil {
			return 0, err
		}

		nextTerm := currentTerm + 1
		_, err = m.conn.Set(m.termPath(), []byte(strconv.FormatInt(nextTerm, 10)), stat.Version)
		if err != nil {
			if errors.Is(err, zk.ErrBadVersion) {
				continue
			}
			return 0, err
		}

		return nextTerm, nil
	}
}

func (m *Manager) leaderAdvanceLogHead(nextIndex int64) error {
	held, err := m.verifyLeadershipStillHeld()
	if err != nil {
		return err
	}
	if !held {
		return errors.New("leader lease not held")
	}

	for {
		data, stat, err := m.conn.Get(m.logHeadPath())
		if err != nil {
			return err
		}

		current, err := parseCounter(data)
		if err != nil {
			return err
		}
		if nextIndex <= current {
			return fmt.Errorf("log head must advance monotonically: current=%d next=%d", current, nextIndex)
		}

		_, err = m.conn.Set(m.logHeadPath(), []byte(strconv.FormatInt(nextIndex, 10)), stat.Version)
		if err != nil {
			if errors.Is(err, zk.ErrBadVersion) {
				continue
			}
			return err
		}

		return nil
	}
}

func (m *Manager) readLogHead() (int64, error) {
	data, _, err := m.conn.Get(m.logHeadPath())
	if err != nil {
		return 0, err
	}
	return parseCounter(data)
}

func parseCounter(data []byte) (int64, error) {
	value := strings.TrimSpace(string(data))
	if value == "" {
		return 0, errors.New("counter value is empty")
	}
	return strconv.ParseInt(value, 10, 64)
}
