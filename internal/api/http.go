package api

import (
	"context"
	"net/http"

	"quorapay/internal/coordination"
	"quorapay/internal/storage"
)

type Config struct {
	NodeID          string
	CORSAllowed     string
	ZKAddr          string
	StoragePath     string
	RequestShutdown func(reason string)
}

type StatusSource interface {
	CurrentStatus() coordination.Status
}

type LedgerStore interface {
	ListPayments(context.Context) ([]storage.Payment, error)
}

type handler struct {
	cfg    Config
	status StatusSource
	ledger LedgerStore
}

func NewHandler(cfg Config, status StatusSource, ledger LedgerStore) http.Handler {
	h := &handler{
		cfg:    cfg,
		status: status,
		ledger: ledger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/status", h.statusHandler)
	mux.HandleFunc("/ledger", h.ledgerHandler)
	mux.HandleFunc("/admin/shutdown", h.shutdownHandler)
	return withCORS(cfg.CORSAllowed, mux)
}
