package coordination

import (
	"errors"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-zookeeper/zk"
)

const (
	RoleLeader   = "LEADER"
	RoleFollower = "FOLLOWER"
	RoleUnknown  = "UNKNOWN"
)

type Config struct {
	NodeID      string
	BaseURL     string
	ZKAddr      string
	ZKRoot      string
	StoragePath string
	Logger      *log.Logger
}

type Status struct {
	NodeID      string `json:"node_id"`
	Role        string `json:"role"`
	LeaderID    string `json:"leader_id"`
	LeaderURL   string `json:"leader_url"`
	ZKAddr      string `json:"zk_addr"`
	StoragePath string `json:"storage_path"`
	ZKError     string `json:"zk_error,omitempty"`
	Timestamp   string `json:"timestamp"`
}

type Manager struct {
	cfg Config

	conn    *zk.Conn
	events  <-chan zk.Event
	started bool
	stopCh  chan struct{}
	doneCh  chan struct{}
	closeMu sync.Once

	mu            sync.RWMutex
	status        Status
	candidatePath string
}

func NewManager(cfg Config) *Manager {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
		cfg.Logger = logger
	}

	return &Manager{
		cfg: cfg,
		status: Status{
			NodeID:      cfg.NodeID,
			Role:        RoleUnknown,
			ZKAddr:      cfg.ZKAddr,
			StoragePath: cfg.StoragePath,
			ZKError:     "zookeeper not connected",
		},
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (m *Manager) Start() error {
	conn, events, err := zk.Connect([]string{m.cfg.ZKAddr}, 5*time.Second)
	if err != nil {
		m.setZKError(err)
		return err
	}

	m.conn = conn
	m.events = events
	m.started = true

	go m.consumeSessionEvents()
	go m.watchElection()

	return nil
}

func (m *Manager) CurrentStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := m.status
	status.Timestamp = time.Now().UTC().Format(time.RFC3339)
	return status
}

func (m *Manager) Close() error {
	m.closeMu.Do(func() {
		close(m.stopCh)
		if m.conn != nil {
			m.conn.Close()
		}
		if m.started {
			<-m.doneCh
		}
	})
	return nil
}

func (m *Manager) consumeSessionEvents() {
	defer close(m.doneCh)

	for {
		select {
		case <-m.stopCh:
			return
		case event, ok := <-m.events:
			if !ok {
				return
			}

			switch event.State {
			case zk.StateHasSession:
				m.clearZKError()
			case zk.StateDisconnected:
				m.setZKError(errors.New("zookeeper disconnected"))
			case zk.StateExpired:
				m.mu.Lock()
				m.candidatePath = ""
				m.mu.Unlock()
				m.setZKError(errors.New("zookeeper session expired"))
			case zk.StateAuthFailed:
				m.setZKError(errors.New("zookeeper authentication failed"))
			}
		}
	}
}

func (m *Manager) watchElection() {
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		if m.conn == nil {
			time.Sleep(2 * time.Second)
			continue
		}

		if err := m.ensureRegistered(); err != nil {
			m.setZKError(err)
			m.cfg.Logger.Printf("coordination reconcile failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if err := m.refreshLeader(); err != nil {
			m.setZKError(err)
			m.cfg.Logger.Printf("leader refresh failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		_, _, eventCh, err := m.conn.ChildrenW(m.electionPath())
		if err != nil {
			m.setZKError(err)
			time.Sleep(2 * time.Second)
			continue
		}

		select {
		case <-m.stopCh:
			return
		case <-time.After(10 * time.Second):
		case _, ok := <-eventCh:
			if !ok {
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
}

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
	if err := m.ensureMemberNode(); err != nil {
		return err
	}
	if err := m.ensureCandidateNode(); err != nil {
		return err
	}

	m.clearZKError()
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

func (m *Manager) refreshLeader() error {
	children, _, err := m.conn.Children(m.electionPath())
	if err != nil {
		return err
	}

	candidates := make([]string, 0, len(children))
	for _, child := range children {
		if strings.HasPrefix(child, "candidate-") {
			candidates = append(candidates, child)
		}
	}

	sort.Strings(candidates)
	if len(candidates) == 0 {
		m.mu.Lock()
		m.status.Role = RoleUnknown
		m.status.LeaderID = ""
		m.status.LeaderURL = ""
		m.mu.Unlock()
		return nil
	}

	leaderPath := m.electionPath() + "/" + candidates[0]
	leaderData, _, err := m.conn.Get(leaderPath)
	if err != nil {
		return err
	}

	leaderID := string(leaderData)
	leaderURL := ""
	memberData, _, err := m.conn.Get(m.membersPath() + "/" + leaderID)
	if err == nil {
		leaderURL = string(memberData)
	}

	m.mu.RLock()
	selfCandidate := m.candidatePath
	m.mu.RUnlock()

	role := RoleFollower
	if selfCandidate != "" && strings.TrimPrefix(selfCandidate, m.electionPath()+"/") == candidates[0] {
		role = RoleLeader
	}

	m.mu.Lock()
	m.status.Role = role
	m.status.LeaderID = leaderID
	m.status.LeaderURL = leaderURL
	m.mu.Unlock()

	return nil
}

func (m *Manager) setZKError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.ZKError = err.Error()
	m.status.Role = RoleUnknown
	m.status.LeaderID = ""
	m.status.LeaderURL = ""
}

func (m *Manager) clearZKError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.ZKError = ""
}

func (m *Manager) membersPath() string {
	return strings.TrimRight(m.cfg.ZKRoot, "/") + "/members"
}

func (m *Manager) memberPath() string {
	return m.membersPath() + "/" + m.cfg.NodeID
}

func (m *Manager) electionPath() string {
	return strings.TrimRight(m.cfg.ZKRoot, "/") + "/election"
}
