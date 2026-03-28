package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quorapay/internal/coordination"
	"quorapay/internal/replication"
	"quorapay/internal/storage"
)

type stubCoordinator struct {
	role         string
	leaderURL    string
	term         int64
	logHead      int64
	followerURLs []string
}

func (s *stubCoordinator) CurrentStatus() coordination.Status {
	return coordination.Status{
		NodeID:    "node-a",
		Role:      s.role,
		LeaderURL: s.leaderURL,
		Term:      s.term,
		LogHead:   s.logHead,
	}
}

func (s *stubCoordinator) GetFollowerURLs() ([]string, error) {
	out := make([]string, len(s.followerURLs))
	copy(out, s.followerURLs)
	return out, nil
}

func (s *stubCoordinator) AdvanceLogHead(nextIndex int64) error {
	s.logHead = nextIndex
	return nil
}

func (s *stubCoordinator) CurrentLogHead() (int64, error) {
	return s.logHead, nil
}

type stubReplicator struct {
	result replication.QuorumReplicationResult
	err    error
}

func (s stubReplicator) ReplicateWithQuorum(_ context.Context, _ replication.LogEntry, _ []string) (replication.QuorumReplicationResult, error) {
	return s.result, s.err
}

func TestPayHandler_NonLeaderRedirects(t *testing.T) {
	coord := &stubCoordinator{
		role:      coordination.RoleFollower,
		leaderURL: "http://leader-node:8080",
		term:      3,
		logHead:   10,
	}

	h := NewHandler(Config{NodeID: "node-b", CORSAllowed: "*"}, coord, newStubLedgerStore(), stubReplicator{})

	resp := performJSONRequest(t, h, http.MethodPost, "/pay", replication.PaymentRequest{
		PaymentID: "pay-redirect",
		Amount:    10,
		Currency:  "USD",
	})

	if resp.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusTemporaryRedirect)
	}

	if got := resp.Header().Get("Location"); got != "http://leader-node:8080/pay" {
		t.Fatalf("location = %q, want %q", got, "http://leader-node:8080/pay")
	}
}

func TestPayHandler_NoLeaderReturns503(t *testing.T) {
	coord := &stubCoordinator{
		role:      coordination.RoleUnknown,
		leaderURL: "",
		term:      0,
		logHead:   10,
	}

	h := NewHandler(Config{NodeID: "node-c", CORSAllowed: "*"}, coord, newStubLedgerStore(), stubReplicator{})

	resp := performJSONRequest(t, h, http.MethodPost, "/pay", replication.PaymentRequest{
		PaymentID: "pay-no-leader",
		Amount:    15,
		Currency:  "USD",
	})

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusServiceUnavailable)
	}
}

func TestPayHandler_DuplicatePaymentReturnsOK(t *testing.T) {
	store := newStubLedgerStore()
	store.payments["pay-dup"] = storage.Payment{PaymentID: "pay-dup", Status: replication.StatusCommitted.String()}

	coord := &stubCoordinator{
		role:    coordination.RoleLeader,
		term:    4,
		logHead: 20,
	}

	h := NewHandler(Config{NodeID: "node-a", CORSAllowed: "*"}, coord, store, stubReplicator{})

	resp := performJSONRequest(t, h, http.MethodPost, "/pay", replication.PaymentRequest{
		PaymentID: "pay-dup",
		Amount:    20,
		Currency:  "USD",
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var out replication.PaymentResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if out.Message != "payment already processed" {
		t.Fatalf("message = %q, want %q", out.Message, "payment already processed")
	}
}

func TestPayHandler_QuorumReachedReturns200(t *testing.T) {
	coord := &stubCoordinator{
		role:         coordination.RoleLeader,
		term:         7,
		logHead:      30,
		followerURLs: []string{"http://node-b:8080", "http://node-c:8080"},
	}

	repl := stubReplicator{result: replication.QuorumReplicationResult{
		QuorumReached: true,
		Committed:     true,
	}}

	h := NewHandler(Config{NodeID: "node-a", CORSAllowed: "*"}, coord, newStubLedgerStore(), repl)

	resp := performJSONRequest(t, h, http.MethodPost, "/pay", replication.PaymentRequest{
		PaymentID: "pay-ok",
		Amount:    42.5,
		Currency:  "USD",
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var out replication.PaymentResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if out.Status != "OK" {
		t.Fatalf("status field = %q, want %q", out.Status, "OK")
	}
}

func TestPayHandler_QuorumNotReachedReturns503(t *testing.T) {
	coord := &stubCoordinator{
		role:    coordination.RoleLeader,
		term:    8,
		logHead: 40,
	}

	repl := stubReplicator{result: replication.QuorumReplicationResult{
		QuorumReached: false,
		Committed:     false,
	}}

	h := NewHandler(Config{NodeID: "node-a", CORSAllowed: "*"}, coord, newStubLedgerStore(), repl)

	resp := performJSONRequest(t, h, http.MethodPost, "/pay", replication.PaymentRequest{
		PaymentID: "pay-no-quorum",
		Amount:    33,
		Currency:  "USD",
	})

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusServiceUnavailable)
	}
}

func TestPayHandler_InvalidBodyReturns400(t *testing.T) {
	coord := &stubCoordinator{
		role:    coordination.RoleLeader,
		term:    2,
		logHead: 5,
	}

	h := NewHandler(Config{NodeID: "node-a", CORSAllowed: "*"}, coord, newStubLedgerStore(), stubReplicator{})

	req := httptest.NewRequest(http.MethodPost, "/pay", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
}
