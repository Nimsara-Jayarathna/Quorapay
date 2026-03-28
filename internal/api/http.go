package api

import (
	"context"
	"net/http"

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

	writeJSON(w, http.StatusNotImplemented, map[string]string{"message": "not implemented"})
}
