package api

import (
	"net/http"
	"time"

	"quorapay/internal/storage"
)

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
