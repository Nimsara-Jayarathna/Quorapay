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

func (s *stubLedgerStore) FailByPaymentID(_ context.Context, paymentID string) error {
	payment, ok := s.payments[paymentID]
	if !ok {
		return storage.ErrPaymentNotFound
	}
	payment.Status = replication.StatusFailed.String()
	s.payments[paymentID] = payment
	return nil
}

func (s *stubLedgerStore) ExistsByPaymentID(_ context.Context, paymentID string) (bool, error) {
	_, exists := s.payments[paymentID]
	return exists, nil
}

func (s *stubLedgerStore) GetPaymentByID(_ context.Context, paymentID string) (storage.Payment, error) {
	payment, exists := s.payments[paymentID]
	if !exists {
		return storage.Payment{}, storage.ErrPaymentNotFound
	}
	return payment, nil
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

func TestCatchUpHandler_NonLeaderNoLeaderURLReturns503(t *testing.T) {
	coord := &stubCoordinator{role: coordination.RoleFollower, leaderURL: ""}
	h := NewHandler(Config{NodeID: "C"}, coord, newStubLedgerStore())

	req := httptest.NewRequest(http.MethodGet, "/internal/catchup", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusServiceUnavailable)
	}
}

func TestCatchUpHandler_NonLeaderRedirectsToLeader(t *testing.T) {
	coord := &stubCoordinator{role: coordination.RoleFollower, leaderURL: "http://leader:8080"}
	h := NewHandler(Config{NodeID: "C"}, coord, newStubLedgerStore())

	req := httptest.NewRequest(http.MethodGet, "/internal/catchup?from_log_index=2", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusTemporaryRedirect)
	}
	loc := resp.Header().Get("Location")
	if loc != "http://leader:8080/internal/catchup?from_log_index=2" {
		t.Fatalf("location = %q, want %q", loc, "http://leader:8080/internal/catchup?from_log_index=2")
	}
}

func TestCatchUpHandler_NilCoordinatorReturns503(t *testing.T) {
	h := NewHandler(Config{NodeID: "C"}, nil, newStubLedgerStore())

	req := httptest.NewRequest(http.MethodGet, "/internal/catchup", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusServiceUnavailable)
	}
}

func TestCatchUpHandler_NoParamDefaultsToZero(t *testing.T) {
	store := newStubLedgerStore()
	store.payments["pay-1"] = storage.Payment{
		PaymentID: "pay-1",
		LogIndex:  1,
		Status:    replication.StatusCommitted.String(),
	}

	coord := &stubCoordinator{role: coordination.RoleLeader}
	h := NewHandler(Config{NodeID: "C"}, coord, store)

	req := httptest.NewRequest(http.MethodGet, "/internal/catchup", nil)
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
	if len(out.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(out.Entries))
	}
	if out.HasMore {
		t.Fatalf("has_more = true, want false")
	}
	if out.TotalAvailable != 1 {
		t.Fatalf("total_available = %d, want 1", out.TotalAvailable)
	}
}

func TestCatchUpHandler_WithFromLogIndex(t *testing.T) {
	store := newStubLedgerStore()
	store.payments["pay-1"] = storage.Payment{
		PaymentID: "pay-1",
		LogIndex:  1,
		Status:    replication.StatusCommitted.String(),
	}
	store.payments["pay-3"] = storage.Payment{
		PaymentID: "pay-3",
		LogIndex:  3,
		Status:    replication.StatusCommitted.String(),
	}

	coord := &stubCoordinator{role: coordination.RoleLeader}
	h := NewHandler(Config{NodeID: "C"}, coord, store)

	req := httptest.NewRequest(http.MethodGet, "/internal/catchup?from_log_index=2", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var out replication.CatchUpResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Entries) != 1 || out.Entries[0].PaymentID != "pay-3" {
		t.Fatalf("entries = %+v, want only pay-3", out.Entries)
	}
	if out.HasMore {
		t.Fatalf("has_more = true, want false")
	}
	if out.TotalAvailable != 1 {
		t.Fatalf("total_available = %d, want 1", out.TotalAvailable)
	}
}

func TestCatchUpHandler_InvalidFromLogIndex(t *testing.T) {
	coord := &stubCoordinator{role: coordination.RoleLeader}
	h := NewHandler(Config{NodeID: "C"}, coord, newStubLedgerStore())

	req := httptest.NewRequest(http.MethodGet, "/internal/catchup?from_log_index=abc", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
}

func TestCatchUpHandler_EmptyLedgerReturnsEmptyEntries(t *testing.T) {
	coord := &stubCoordinator{role: coordination.RoleLeader}
	h := NewHandler(Config{NodeID: "C"}, coord, newStubLedgerStore())

	req := httptest.NewRequest(http.MethodGet, "/internal/catchup", nil)
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
	if out.Entries == nil || len(out.Entries) != 0 {
		// Slice len == 0 allows nil slice, which standard json unmarshal converts properly
		// if we strictly check size. But let's just make sure it's 0.
		if len(out.Entries) != 0 {
			t.Fatalf("expected 0 entries, got %d", len(out.Entries))
		}
	}
}

func TestCatchUpHandler_LimitCapsResults(t *testing.T) {
	store := newStubLedgerStore()
	store.payments["pay-1"] = storage.Payment{PaymentID: "pay-1", LogIndex: 1, Status: replication.StatusCommitted.String()}
	store.payments["pay-2"] = storage.Payment{PaymentID: "pay-2", LogIndex: 2, Status: replication.StatusCommitted.String()}
	store.payments["pay-3"] = storage.Payment{PaymentID: "pay-3", LogIndex: 3, Status: replication.StatusCommitted.String()}

	coord := &stubCoordinator{role: coordination.RoleLeader}
	h := NewHandler(Config{NodeID: "C"}, coord, store)

	req := httptest.NewRequest(http.MethodGet, "/internal/catchup?limit=2", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var out replication.CatchUpResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(out.Entries))
	}
	if !out.HasMore {
		t.Fatalf("has_more = false, want true")
	}
	if out.TotalAvailable != 3 {
		t.Fatalf("total_available = %d, want 3", out.TotalAvailable)
	}
}

func TestCatchUpHandler_InvalidLimitReturns400(t *testing.T) {
	coord := &stubCoordinator{role: coordination.RoleLeader}
	h := NewHandler(Config{NodeID: "C"}, coord, newStubLedgerStore())

	req := httptest.NewRequest(http.MethodGet, "/internal/catchup?limit=abc", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
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

func TestStatusIncludesLamportAndClockSkew(t *testing.T) {
	store := newStubLedgerStore()
	h := NewHandler(Config{NodeID: "B", CORSAllowed: "*"}, stubStatusSource{}, store)

	// Drive Lamport + skew state via one append.
	body := replication.AppendEntriesRequest{
		LeaderID: "A",
		Term:     1,
		Entries: []replication.LogEntry{{
			LogIndex:     22,
			Term:         1,
			LeaderID:     "A",
			PaymentID:    "pay-status-sync",
			Amount:       5,
			Currency:     "USD",
			Status:       replication.StatusPending,
			LogicalTime:  8,
			PhysicalTime: time.Now().Add(-400 * time.Millisecond).UnixNano(),
		}},
	}
	appendResp := performJSONRequest(t, h, http.MethodPost, "/internal/append", body)
	if appendResp.Code != http.StatusOK {
		t.Fatalf("append status = %d, want %d", appendResp.Code, http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", resp.Code, http.StatusOK)
	}

	var out map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	lamport, ok := out["lamport_time"].(float64)
	if !ok || lamport <= 0 {
		t.Fatalf("lamport_time = %v, want > 0", out["lamport_time"])
	}
	skew, ok := out["clock_skew_ms"].(float64)
	if !ok || skew <= 0 {
		t.Fatalf("clock_skew_ms = %v, want > 0", out["clock_skew_ms"])
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
