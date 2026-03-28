package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quorapay/internal/coordination"
	"quorapay/internal/replication"
	"quorapay/internal/storage"
)

type stubStatusSource struct{}

func (stubStatusSource) CurrentStatus() coordination.Status {
	return coordination.Status{NodeID: "B"}
}

type stubLedgerStore struct {
	payments map[string]storage.Payment
}

func newStubLedgerStore() *stubLedgerStore {
	return &stubLedgerStore{payments: map[string]storage.Payment{}}
}

func (s *stubLedgerStore) ListPayments(context.Context) ([]storage.Payment, error) {
	items := make([]storage.Payment, 0, len(s.payments))
	for _, p := range s.payments {
		items = append(items, p)
	}
	return items, nil
}

func (s *stubLedgerStore) AppendPending(_ context.Context, entry replication.LogEntry) error {
	if _, exists := s.payments[entry.PaymentID]; exists {
		return storage.ErrDuplicatePaymentID
	}

	s.payments[entry.PaymentID] = storage.Payment{
		PaymentID:   entry.PaymentID,
		LogIndex:    entry.LogIndex,
		Amount:      entry.Amount,
		Currency:    entry.Currency,
		Status:      replication.StatusPending.String(),
		LogicalTime: entry.LogicalTime,
		ReceivedBy:  entry.ReceivedBy,
		ProcessedBy: entry.LeaderID,
	}
	return nil
}

func (s *stubLedgerStore) ListCommittedAfter(_ context.Context, logIndex int64) ([]storage.Payment, error) {
	items := make([]storage.Payment, 0, len(s.payments))
	for _, p := range s.payments {
		if p.Status == replication.StatusCommitted.String() && p.LogIndex > logIndex {
			items = append(items, p)
		}
	}
	return items, nil
}

func (s *stubLedgerStore) CommitByPaymentID(_ context.Context, paymentID string) error {
	payment, ok := s.payments[paymentID]
	if !ok {
		return storage.ErrPaymentNotFound
	}
	payment.Status = replication.StatusCommitted.String()
	s.payments[paymentID] = payment
	return nil
}

func (s *stubLedgerStore) ExistsByPaymentID(_ context.Context, paymentID string) (bool, error) {
	_, exists := s.payments[paymentID]
	return exists, nil
}

func TestInternalAppendSuccess(t *testing.T) {
	h := NewHandler(Config{NodeID: "B", CORSAllowed: "*"}, stubStatusSource{}, newStubLedgerStore())

	body := replication.AppendEntriesRequest{
		LeaderID: "A",
		Term:     1,
		Entries: []replication.LogEntry{{
			LogIndex:     10,
			Term:         1,
			LeaderID:     "A",
			PaymentID:    "pay-1",
			Amount:       25,
			Currency:     "USD",
			Status:       replication.StatusPending,
			PhysicalTime: time.Now().UnixNano(),
		}},
	}

	resp := performJSONRequest(t, h, http.MethodPost, "/internal/append", body)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var ack replication.AppendEntriesResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &ack); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !ack.Success {
		t.Fatalf("ack success = false, want true")
	}
}

func TestInternalAppendDuplicateIsIdempotent(t *testing.T) {
	store := newStubLedgerStore()
	h := NewHandler(Config{NodeID: "B", CORSAllowed: "*"}, stubStatusSource{}, store)

	body := replication.AppendEntriesRequest{
		LeaderID: "A",
		Term:     2,
		Entries: []replication.LogEntry{{
			LogIndex:     11,
			Term:         2,
			LeaderID:     "A",
			PaymentID:    "pay-dup",
			Amount:       50,
			Currency:     "USD",
			Status:       replication.StatusPending,
			PhysicalTime: time.Now().UnixNano(),
		}},
	}

	first := performJSONRequest(t, h, http.MethodPost, "/internal/append", body)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusOK)
	}

	second := performJSONRequest(t, h, http.MethodPost, "/internal/append", body)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusOK)
	}

	var ack replication.AppendEntriesResponse
	if err := json.Unmarshal(second.Body.Bytes(), &ack); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !ack.Success {
		t.Fatalf("duplicate append should be successful idempotent ack")
	}
}

func TestInternalCommitSuccess(t *testing.T) {
	store := newStubLedgerStore()
	store.payments["pay-commit"] = storage.Payment{PaymentID: "pay-commit", Status: replication.StatusPending.String()}
	h := NewHandler(Config{NodeID: "B", CORSAllowed: "*"}, stubStatusSource{}, store)

	body := replication.CommitRequest{PaymentID: "pay-commit", LogIndex: 12}
	resp := performJSONRequest(t, h, http.MethodPost, "/internal/commit", body)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var ack replication.CommitResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &ack); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !ack.Success {
		t.Fatalf("ack success = false, want true")
	}
}

func TestInternalCommitAlreadyCommittedIsIdempotent(t *testing.T) {
	store := newStubLedgerStore()
	store.payments["pay-committed"] = storage.Payment{PaymentID: "pay-committed", Status: replication.StatusCommitted.String()}
	h := NewHandler(Config{NodeID: "B", CORSAllowed: "*"}, stubStatusSource{}, store)

	body := replication.CommitRequest{PaymentID: "pay-committed", LogIndex: 13}
	resp := performJSONRequest(t, h, http.MethodPost, "/internal/commit", body)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
}

func TestInternalCommitMissingPayment(t *testing.T) {
	h := NewHandler(Config{NodeID: "B", CORSAllowed: "*"}, stubStatusSource{}, newStubLedgerStore())

	body := replication.CommitRequest{PaymentID: "pay-missing", LogIndex: 14}
	resp := performJSONRequest(t, h, http.MethodPost, "/internal/commit", body)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNotFound)
	}

	var ack replication.CommitResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &ack); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ack.Success {
		t.Fatalf("missing payment commit should return success=false")
	}
}

func TestInternalCatchUpLeaderReturnsCommittedEntries(t *testing.T) {
	store := newStubLedgerStore()
	store.payments["pay-1"] = storage.Payment{
		PaymentID:   "pay-1",
		LogIndex:    3,
		Amount:      10,
		Currency:    "USD",
		Status:      replication.StatusCommitted.String(),
		ReceivedBy:  "A",
		ProcessedBy: "C",
	}
	store.payments["pay-2"] = storage.Payment{
		PaymentID:   "pay-2",
		LogIndex:    5,
		Amount:      12,
		Currency:    "USD",
		Status:      replication.StatusCommitted.String(),
		ReceivedBy:  "B",
		ProcessedBy: "C",
	}

	coord := &stubCoordinator{role: coordination.RoleLeader}
	h := NewHandler(Config{NodeID: "C", CORSAllowed: "*"}, coord, store)

	req := httptest.NewRequest(http.MethodGet, "/internal/catchup?from_log_index=3", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var out replication.CatchUpResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !out.Success {
		t.Fatalf("success = false, want true")
	}
	if len(out.Entries) != 1 || out.Entries[0].PaymentID != "pay-2" {
		t.Fatalf("entries = %+v, want only pay-2", out.Entries)
	}
}

func TestInternalAppendUpdatesLamportLogicalTime(t *testing.T) {
	store := newStubLedgerStore()
	h := NewHandler(Config{NodeID: "B", CORSAllowed: "*"}, stubStatusSource{}, store)

	body := replication.AppendEntriesRequest{
		LeaderID: "A",
		Term:     1,
		Entries: []replication.LogEntry{{
			LogIndex:     20,
			Term:         1,
			LeaderID:     "A",
			PaymentID:    "pay-lamport",
			Amount:       5,
			Currency:     "USD",
			Status:       replication.StatusPending,
			LogicalTime:  10,
			PhysicalTime: time.Now().UnixNano(),
		}},
	}

	resp := performJSONRequest(t, h, http.MethodPost, "/internal/append", body)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	got, ok := store.payments["pay-lamport"]
	if !ok {
		t.Fatalf("payment not stored")
	}
	if got.LogicalTime <= 10 {
		t.Fatalf("logical_time = %d, want > 10 (receive rule)", got.LogicalTime)
	}
}

func TestInternalAppendRejectsTooOldPhysicalTime(t *testing.T) {
	store := newStubLedgerStore()
	h := NewHandler(Config{NodeID: "B", CORSAllowed: "*"}, stubStatusSource{}, store)

	body := replication.AppendEntriesRequest{
		LeaderID: "A",
		Term:     1,
		Entries: []replication.LogEntry{{
			LogIndex:     21,
			Term:         1,
			LeaderID:     "A",
			PaymentID:    "pay-old-time",
			Amount:       5,
			Currency:     "USD",
			Status:       replication.StatusPending,
			LogicalTime:  10,
			PhysicalTime: time.Now().Add(-5 * time.Second).UnixNano(),
		}},
	}

	resp := performJSONRequest(t, h, http.MethodPost, "/internal/append", body)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
}

func performJSONRequest(t *testing.T, h http.Handler, method string, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	return resp
}
