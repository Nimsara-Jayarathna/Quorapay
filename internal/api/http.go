package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"quorapay/internal/coordination"
	"quorapay/internal/replication"
	"quorapay/internal/storage"
)

type Config struct {
	NodeID          string
	CORSAllowed     string
	ZKAddr          string
	StoragePath     string
	RequestShutdown func(reason string)
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

type LedgerStore interface {
	ListPayments(context.Context) ([]storage.Payment, error)
	AppendPending(context.Context, replication.LogEntry) error
	CommitByPaymentID(context.Context, string) error
	ExistsByPaymentID(context.Context, string) (bool, error)
}

type handler struct {
	cfg         Config
	status      interface{ CurrentStatus() coordination.Status }
	coordinator Coordinator
	ledger      LedgerStore
	replicator  Replicator
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

	h := &handler{
		cfg:         cfg,
		status:      status,
		coordinator: coord,
		ledger:      ledger,
		replicator:  repl,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/status", h.statusHandler)
	mux.HandleFunc("/ledger", h.ledgerHandler)
	mux.HandleFunc("/pay", h.payHandler)
	mux.HandleFunc("/internal/append", h.internalAppendHandler)
	mux.HandleFunc("/internal/commit", h.internalCommitHandler)
	mux.HandleFunc("/admin/shutdown", h.shutdownHandler)
	return withCORS(cfg.CORSAllowed, mux)
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
	if status.Role != coordination.RoleLeader {
		if status.LeaderURL != "" {
			http.Redirect(w, r, strings.TrimRight(status.LeaderURL, "/")+"/pay", http.StatusTemporaryRedirect)
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

	exists, err := h.ledger.ExistsByPaymentID(r.Context(), req.PaymentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "failed to check payment id"})
		return
	}
	if exists {
		writeJSON(w, http.StatusOK, replication.PaymentResponse{
			Status:    "OK",
			PaymentID: req.PaymentID,
			Message:   "payment already processed",
		})
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
		PaymentID:    req.PaymentID,
		Amount:       req.Amount,
		Currency:     req.Currency,
		Status:       replication.StatusPending,
		PhysicalTime: time.Now().UnixNano(),
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
