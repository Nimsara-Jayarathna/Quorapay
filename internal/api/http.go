package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"quorapay/internal/coordination"
	"quorapay/internal/replication"
	"quorapay/internal/storage"
	"quorapay/internal/timesync"
)

const receivedByHeader = "X-Quorapay-Received-By"

type Config struct {
	NodeID           string
	CORSAllowed      string
	ZKAddr           string
	StoragePath      string
	SkewWarnMS       int64
	SkewRejectMS     int64
	MaxMessageAgeMS  int64
	MaxFutureDriftMS int64
	LeaderHTTPClient *http.Client
}

type Coordinator interface {
	CurrentStatus() coordination.Status
	GetFollowerURLs() ([]string, error)
	AdvanceLogHead(nextIndex int64) error
	CurrentLogHead() (int64, error)
}

type Replicator interface {
	ReplicateWithQuorum(ctx context.Context, entry replication.LogEntry, followerURLs []string) (replication.QuorumReplicationResult, error)
}

type MemberDiscovery interface {
	GetMembers() ([]coordination.Member, error)
}

type LedgerStore interface {
	ListPayments(context.Context) ([]storage.Payment, error)
	ListCommittedAfter(context.Context, int64) ([]storage.Payment, error)
	AppendPending(context.Context, replication.LogEntry) error
	CommitByPaymentID(context.Context, string) error
	ExistsByPaymentID(context.Context, string) (bool, error)
	GetPaymentByID(context.Context, string) (storage.Payment, error)
}

type handler struct {
	cfg              Config
	status           interface{ CurrentStatus() coordination.Status }
	coordinator      Coordinator
	ledger           LedgerStore
	replicator       Replicator
	discovery        MemberDiscovery
	leaderHTTPClient *http.Client
	lamportClock     *timesync.LamportClock
	skewTracker      *timesync.SkewTracker
	skewWarn         time.Duration
	skewReject       time.Duration
	maxMessageAge    time.Duration
	maxFutureDrift   time.Duration
}

func NewHandler(cfg Config, status interface{ CurrentStatus() coordination.Status }, ledger LedgerStore, replicator ...Replicator) http.Handler {
	var repl Replicator
	if len(replicator) > 0 {
		repl = replicator[0]
	}

	var coord Coordinator
	if c, ok := status.(Coordinator); ok {
		coord = c
	}
	var discovery MemberDiscovery
	if d, ok := status.(MemberDiscovery); ok {
		discovery = d
	}

	leaderClient := cfg.LeaderHTTPClient
	if leaderClient == nil {
		leaderClient = &http.Client{Timeout: 5 * time.Second}
	}

	h := &handler{
		cfg:              cfg,
		status:           status,
		coordinator:      coord,
		ledger:           ledger,
		replicator:       repl,
		discovery:        discovery,
		leaderHTTPClient: leaderClient,
		lamportClock:     timesync.NewLamportClock(),
		skewTracker:      timesync.NewSkewTracker(),
		skewWarn:         durationOrDefaultMS(cfg.SkewWarnMS, 300*time.Millisecond),
		skewReject:       durationOrDefaultMS(cfg.SkewRejectMS, 500*time.Millisecond),
		maxMessageAge:    durationOrDefaultMS(cfg.MaxMessageAgeMS, 2*time.Second),
		maxFutureDrift:   durationOrDefaultMS(cfg.MaxFutureDriftMS, 500*time.Millisecond),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/status", h.statusHandler)
	mux.HandleFunc("/ledger", h.ledgerHandler)
	mux.HandleFunc("/pay", h.payHandler)
	mux.HandleFunc("/internal/append", h.internalAppendHandler)
	mux.HandleFunc("/internal/commit", h.internalCommitHandler)
	mux.HandleFunc("/internal/catchup", h.internalCatchUpHandler)
	mux.HandleFunc("/cluster/nodes", h.clusterNodesHandler)
	return withCORS(cfg.CORSAllowed, mux)
}

func (h *handler) clusterNodesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}
	if h.discovery == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "member discovery is not available"})
		return
	}
	members, err := h.discovery.GetMembers()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(members),
		"items": members,
	})
}

func durationOrDefaultMS(valueMS int64, fallback time.Duration) time.Duration {
	if valueMS <= 0 {
		return fallback
	}
	return time.Duration(valueMS) * time.Millisecond
}

func (h *handler) payHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}

	if h.coordinator == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "coordinator is not configured"})
		return
	}

	status := h.coordinator.CurrentStatus()
	receivedBy := strings.TrimSpace(r.Header.Get(receivedByHeader))
	if receivedBy == "" {
		receivedBy = h.cfg.NodeID
	}
	if status.Role != coordination.RoleLeader {
		if status.LeaderURL != "" {
			h.forwardPayToLeader(w, r, status.LeaderURL, receivedBy)
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "no leader available"})
		return
	}

	var req replication.PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}
	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return
	}

	payment, err := h.ledger.GetPaymentByID(r.Context(), req.PaymentID)
	if err == nil {
		if payment.Status == replication.StatusCommitted.String() {
			writeJSON(w, http.StatusOK, replication.PaymentResponse{
				Status:    "OK",
				PaymentID: req.PaymentID,
				Message:   "payment already processed",
			})
			return
		}
		if payment.Status == replication.StatusPending.String() {
			writeJSON(w, http.StatusConflict, map[string]string{
				"message": "payment is pending — quorum was not reached on previous attempt, please retry",
			})
			return
		}
	} else if !errors.Is(err, storage.ErrPaymentNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "failed to check payment id"})
		return
	}

	currentLogHead, err := h.coordinator.CurrentLogHead()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "failed to assign log index"})
		return
	}
	nextIndex := currentLogHead + 1
	if err := h.coordinator.AdvanceLogHead(nextIndex); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "failed to assign log index"})
		return
	}

	entry := replication.LogEntry{
		LogIndex:     nextIndex,
		Term:         status.Term,
		LeaderID:     status.NodeID,
		ReceivedBy:   receivedBy,
		PaymentID:    req.PaymentID,
		Amount:       req.Amount,
		Currency:     req.Currency,
		Status:       replication.StatusPending,
		PhysicalTime: time.Now().UnixNano(),
		LogicalTime:  int64(h.lamportClock.Send()),
	}

	followerURLs, err := h.coordinator.GetFollowerURLs()
	if err != nil {
		log.Printf("failed to fetch follower URLs: %v", err)
		followerURLs = []string{}
	}

	if h.replicator == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "replicator is not configured"})
		return
	}

	result, err := h.replicator.ReplicateWithQuorum(r.Context(), entry, followerURLs)
	if err != nil {
		if !result.QuorumReached {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "quorum not reached"})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": err.Error()})
		return
	}

	if !result.QuorumReached {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "quorum not reached"})
		return
	}

	writeJSON(w, http.StatusOK, replication.PaymentResponse{
		Status:    "OK",
		PaymentID: entry.PaymentID,
		LogIndex:  entry.LogIndex,
		Term:      entry.Term,
		LeaderID:  entry.LeaderID,
	})
}

func (h *handler) forwardPayToLeader(w http.ResponseWriter, r *http.Request, leaderURL string, receivedBy string) {
	endpoint := strings.TrimRight(leaderURL, "/") + "/pay"
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "failed to reach leader"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(receivedByHeader, receivedBy)

	resp, err := h.leaderHTTPClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "failed to reach leader"})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "failed to read leader response"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}
