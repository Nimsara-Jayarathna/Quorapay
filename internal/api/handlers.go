package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"quorapay/internal/coordination"
	"quorapay/internal/replication"
	"quorapay/internal/storage"
)

const warnClockSkew = 300 * time.Millisecond

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
		if entry.LogicalTime < 0 {
			writeJSON(w, http.StatusBadRequest, replication.AppendEntriesResponse{
				Success: false,
				Term:    req.Term,
				Message: "logical_time cannot be negative",
			})
			return
		}
		if entry.PhysicalTime <= 0 {
			writeJSON(w, http.StatusBadRequest, replication.AppendEntriesResponse{
				Success: false,
				Term:    req.Term,
				Message: "physical_time is required",
			})
			return
		}

		msgTime := time.Unix(0, entry.PhysicalTime)
		offset := time.Since(msgTime)
		h.skewTracker.Record(entry.LeaderID, offset)
		if err := h.timeValidator.Validate(entry.LeaderID, msgTime); err != nil {
			writeJSON(w, http.StatusBadRequest, replication.AppendEntriesResponse{
				Success: false,
				Term:    req.Term,
				Message: err.Error(),
			})
			return
		}
		if offset < 0 {
			offset = -offset
		}
		if offset > warnClockSkew {
			log.Printf("clock skew warning leader_id=%s observed_offset=%s payment_id=%s", entry.LeaderID, offset, entry.PaymentID)
		}

		// Lamport receive rule: local = max(local, remote) + 1
		entry.LogicalTime = int64(h.lamportClock.Receive(uint64(entry.LogicalTime)))

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

func (h *handler) internalCatchUpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}

	if h.coordinator == nil {
		writeJSON(w, http.StatusServiceUnavailable, replication.CatchUpResponse{
			Success: false,
			Message: "coordinator is not configured",
		})
		return
	}

	status := h.coordinator.CurrentStatus()
	if status.Role != coordination.RoleLeader {
		writeJSON(w, http.StatusServiceUnavailable, replication.CatchUpResponse{
			Success: false,
			Message: "catch-up source must be leader",
		})
		return
	}

	fromRaw := r.URL.Query().Get("from_log_index")
	if fromRaw == "" {
		fromRaw = "0"
	}
	fromLogIndex, err := strconv.ParseInt(fromRaw, 10, 64)
	if err != nil || fromLogIndex < 0 {
		writeJSON(w, http.StatusBadRequest, replication.CatchUpResponse{
			Success: false,
			Message: "from_log_index must be a non-negative integer",
		})
		return
	}

	items, err := h.ledger.ListCommittedAfter(r.Context(), fromLogIndex)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, replication.CatchUpResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	entries := make([]replication.LogEntry, 0, len(items))
	for _, p := range items {
		entries = append(entries, replication.LogEntry{
			LogIndex:     p.LogIndex,
			LeaderID:     p.ProcessedBy,
			ReceivedBy:   p.ReceivedBy,
			PaymentID:    p.PaymentID,
			Amount:       p.Amount,
			Currency:     p.Currency,
			Status:       replication.StatusCommitted,
			PhysicalTime: p.PhysicalTime,
			LogicalTime:  p.LogicalTime,
		})
	}

	writeJSON(w, http.StatusOK, replication.CatchUpResponse{
		Success: true,
		Entries: entries,
	})
}
