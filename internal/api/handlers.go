package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"quorapay/internal/replication"
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

func (h *handler) internalAppendHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}

	var req replication.AppendEntriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, replication.AppendEntriesResponse{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if len(req.Entries) == 0 {
		writeJSON(w, http.StatusBadRequest, replication.AppendEntriesResponse{
			Success: false,
			Term:    req.Term,
			Message: "entries cannot be empty",
		})
		return
	}

	var lastIndex int64
	for _, entry := range req.Entries {
		if entry.PaymentID == "" {
			writeJSON(w, http.StatusBadRequest, replication.AppendEntriesResponse{
				Success: false,
				Term:    req.Term,
				Message: "payment_id is required",
			})
			return
		}
		if entry.LogIndex < 0 {
			writeJSON(w, http.StatusBadRequest, replication.AppendEntriesResponse{
				Success: false,
				Term:    req.Term,
				Message: "log_index cannot be negative",
			})
			return
		}

		if err := h.ledger.AppendPending(r.Context(), entry); err != nil {
			if errors.Is(err, storage.ErrDuplicatePaymentID) {
				lastIndex = entry.LogIndex
				continue
			}

			writeJSON(w, http.StatusInternalServerError, replication.AppendEntriesResponse{
				Success:      false,
				Term:         req.Term,
				LastLogIndex: lastIndex,
				Message:      err.Error(),
			})
			return
		}

		lastIndex = entry.LogIndex
	}

	writeJSON(w, http.StatusOK, replication.AppendEntriesResponse{
		Success:      true,
		Term:         req.Term,
		LastLogIndex: lastIndex,
		Message:      "append applied",
	})
}

func (h *handler) internalCommitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}

	var req replication.CommitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, replication.CommitResponse{
			Success: false,
			Message: "invalid request body",
		})
		return
	}

	if req.PaymentID == "" {
		writeJSON(w, http.StatusBadRequest, replication.CommitResponse{
			Success: false,
			Message: "payment_id is required",
		})
		return
	}

	err := h.ledger.CommitByPaymentID(r.Context(), req.PaymentID)
	if err != nil {
		if errors.Is(err, storage.ErrPaymentNotFound) {
			writeJSON(w, http.StatusNotFound, replication.CommitResponse{
				Success: false,
				Message: err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusInternalServerError, replication.CommitResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, replication.CommitResponse{
		Success: true,
		Message: "commit applied",
	})
}
