package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"quorapay/internal/coordination"
	"quorapay/internal/storage"
)

type Config struct {
	NodeID      string
	CORSAllowed string
	ZKAddr      string
	StoragePath string
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

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) statusHandler(w http.ResponseWriter, _ *http.Request) {
	status := h.status.CurrentStatus()
	status.Timestamp = time.Now().UTC().Format(time.RFC3339)
	if status.NodeID == "" {
		status.NodeID = h.cfg.NodeID
	}
	if status.ZKAddr == "" {
		status.ZKAddr = h.cfg.ZKAddr
	}
	if status.StoragePath == "" {
		status.StoragePath = h.cfg.StoragePath
	}

	writeJSON(w, http.StatusOK, status)
}

func (h *handler) ledgerHandler(w http.ResponseWriter, r *http.Request) {
	items, err := h.ledger.ListPayments(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Count int               `json:"count"`
		Items []storage.Payment `json:"items"`
	}{
		Count: len(items),
		Items: items,
	})
}

func (h *handler) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}

	if h.cfg.RequestShutdown == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"message": "shutdown is not configured"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"message": "shutdown scheduled",
		"node_id": h.cfg.NodeID,
	})

	go func() {
		time.Sleep(150 * time.Millisecond)
		h.cfg.RequestShutdown("requested by /admin/shutdown")
	}()
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		log.Printf("json encode failed: %v", err)
	}
}

func withCORS(allowedOrigins string, next http.Handler) http.Handler {
	allowed := parseOrigins(allowedOrigins)
	allowAny := len(allowed) == 1 && allowed[0] == "*"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowAny {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && isAllowedOrigin(origin, allowed) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parseOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			origins = append(origins, part)
		}
	}
	return origins
}

func isAllowedOrigin(origin string, allowed []string) bool {
	for _, item := range allowed {
		if item == origin {
			return true
		}
	}
	return false
}
