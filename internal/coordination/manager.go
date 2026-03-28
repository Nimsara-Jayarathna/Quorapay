package coordination

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-zookeeper/zk"
)

const (
	RoleLeader   = "LEADER"
	RoleFollower = "FOLLOWER"
	RoleUnknown  = "UNKNOWN"
	rejoinedHold = 3 * time.Second
)

type Config struct {
	NodeID      string
	BaseURL     string
	ZKAddr      string
	ZKRoot      string
	StoragePath string
	Logger      *log.Logger
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
	rejoinedSince  time.Time
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
			FaultState:  FaultStateRecovering,
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

func (m *Manager) AdvanceLogHead(nextIndex int64) error {
	return m.leaderAdvanceLogHead(nextIndex)
}

func (m *Manager) CurrentLogHead() (int64, error) {
	return m.readLogHead()
}

func (m *Manager) GetFollowerURLs() ([]string, error) {
	if m.conn == nil || !m.started {
		return []string{}, fmt.Errorf("zookeeper connection is not initialized")
	}

	children, _, err := m.conn.Children(m.membersPath())
	if err != nil {
		return []string{}, fmt.Errorf("read cluster members: %w", err)
	}

	followers := make([]string, 0, len(children))
	for _, child := range children {
		data, _, getErr := m.conn.Get(m.membersPath() + "/" + child)
		if getErr != nil {
			if errors.Is(getErr, zk.ErrNoNode) {
				continue
			}
			return []string{}, fmt.Errorf("read member %s data: %w", child, getErr)
		}

		memberURL := string(data)
		if memberURL == "" || memberURL == m.cfg.BaseURL {
			continue
		}

		followers = append(followers, memberURL)
	}

	return followers, nil
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
				if err := m.setFaultState(FaultStateRecovering, "zookeeper session established"); err != nil {
					m.cfg.Logger.Printf("fault state transition skipped: %v", err)
				}
			case zk.StateDisconnected:
				m.setZKError(errors.New("zookeeper disconnected"))
				if err := m.setFaultState(FaultStateFailed, "zookeeper disconnected"); err != nil {
					m.cfg.Logger.Printf("fault state transition skipped: %v", err)
				}
			case zk.StateExpired:
				m.mu.Lock()
				m.candidatePath = ""
				m.eligibleSince = time.Time{}
				m.mu.Unlock()
				m.setZKError(errors.New("zookeeper session expired"))
				if err := m.setFaultState(FaultStateFailed, "zookeeper session expired"); err != nil {
					m.cfg.Logger.Printf("fault state transition skipped: %v", err)
				}
			case zk.StateAuthFailed:
				m.setZKError(errors.New("zookeeper authentication failed"))
				if err := m.setFaultState(FaultStateFailed, "zookeeper authentication failed"); err != nil {
					m.cfg.Logger.Printf("fault state transition skipped: %v", err)
				}
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

	m.mu.RLock()
	currentFaultState := m.status.FaultState
	joinedAt := m.rejoinedSince
	m.mu.RUnlock()

	if currentFaultState == FaultStateRecovering {
		if err := m.setFaultState(FaultStateRejoined, "node rejoined cluster"); err != nil {
			m.cfg.Logger.Printf("fault state transition skipped: %v", err)
		}
	} else if currentFaultState == FaultStateRejoined && !joinedAt.IsZero() && time.Since(joinedAt) >= rejoinedHold {
		if err := m.setFaultState(FaultStateHealthy, "node operating normally"); err != nil {
			m.cfg.Logger.Printf("fault state transition skipped: %v", err)
		}
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
