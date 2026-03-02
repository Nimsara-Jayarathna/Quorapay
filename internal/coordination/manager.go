package coordination

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
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

type leaderLease struct {
	LeaderID  string `json:"leader_id"`
	LeaderURL string `json:"leader_url"`
	Term      int64  `json:"term"`
	Since     string `json:"since,omitempty"`
	LeaseID   string `json:"lease_id,omitempty"`
}

type Manager struct {
	cfg Config

	conn    *zk.Conn
	events  <-chan zk.Event
	started bool
	stopCh  chan struct{}
	doneCh  chan struct{}
	closeMu sync.Once

	mu             sync.RWMutex
	status         Status
	candidatePath  string
	eligibleSince  time.Time
	lastLoopStart  time.Time
	lastKnownLease leaderLease
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
	go m.watchCoordination()

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
				m.eligibleSince = time.Time{}
				m.mu.Unlock()
				m.setZKError(errors.New("zookeeper session expired"))
			case zk.StateAuthFailed:
				m.setZKError(errors.New("zookeeper authentication failed"))
			}
		}
	}
}

func (m *Manager) watchCoordination() {
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		m.recordStatusRefreshTick()

		if m.conn == nil {
			time.Sleep(2 * time.Second)
			continue
		}

		if err := m.reconcile(); err != nil {
			m.setZKError(err)
			m.cfg.Logger.Printf("coordination reconcile failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if err := m.waitForCoordinationChange(); err != nil && !errors.Is(err, zk.ErrClosing) {
			m.setZKError(err)
			time.Sleep(2 * time.Second)
		}
	}
}

func (m *Manager) reconcile() error {
	if err := m.ensureRegistered(); err != nil {
		return err
	}

	eligible, err := m.determineLeaderEligible()
	if err != nil {
		return err
	}

	if eligible {
		if _, _, err := m.tryAcquireLeaderLease(); err != nil {
			return err
		}
	} else {
		m.clearEligibility()
	}

	if err := m.refreshStatusFromLease(); err != nil {
		if errors.Is(err, zk.ErrNoNode) {
			m.setNoLeaderStatus()
			return nil
		}
		return err
	}

	m.clearZKError()
	return nil
}

func (m *Manager) waitForCoordinationChange() error {
	_, _, electionWatch, err := m.conn.ChildrenW(m.electionPath())
	if err != nil {
		return err
	}

	_, _, leaderWatch, err := m.conn.ExistsW(m.leaderPath())
	if err != nil {
		return err
	}

	select {
	case <-m.stopCh:
		return nil
	case <-time.After(5 * time.Second):
		return nil
	case _, ok := <-electionWatch:
		if !ok {
			time.Sleep(200 * time.Millisecond)
		}
		return nil
	case _, ok := <-leaderWatch:
		if !ok {
			time.Sleep(200 * time.Millisecond)
		}
		return nil
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
	m.status.Role = role
	m.status.LeaderID = lease.LeaderID
	m.status.LeaderURL = lease.LeaderURL
	m.status.Term = lease.Term
	m.status.LogHead = logHead
	m.status.ZKError = ""
	m.lastKnownLease = lease
	m.mu.Unlock()

	return nil
}

func (m *Manager) releaseLeaderLeaseIfOwned() {
	held, err := m.verifyLeadershipStillHeld()
	if err != nil || !held {
		return
	}
	_ = m.conn.Delete(m.leaderPath(), -1)
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

func (m *Manager) membersPath() string {
	return strings.TrimRight(m.cfg.ZKRoot, "/") + "/members"
}

func (m *Manager) memberPath() string {
	return m.membersPath() + "/" + m.cfg.NodeID
}

func (m *Manager) electionPath() string {
	return strings.TrimRight(m.cfg.ZKRoot, "/") + "/election"
}

func (m *Manager) leaderPath() string {
	return strings.TrimRight(m.cfg.ZKRoot, "/") + "/leader"
}

func (m *Manager) termPath() string {
	return strings.TrimRight(m.cfg.ZKRoot, "/") + "/term"
}

func (m *Manager) logHeadPath() string {
	return strings.TrimRight(m.cfg.ZKRoot, "/") + "/log_head"
}

func parseCounter(data []byte) (int64, error) {
	value := strings.TrimSpace(string(data))
	if value == "" {
		return 0, errors.New("counter value is empty")
	}
	return strconv.ParseInt(value, 10, 64)
}
