package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (s stubReplicator) ReplicateWithQuorum(_ context.Context, _ replication.LogEntry, _ []string) (replication.QuorumReplicationResult, error) {
	return s.result, s.err
}

func TestPayHandler_NonLeaderForwardsToLeader(t *testing.T) {
	leaderClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", req.Method)
			}
			if req.URL.String() != "http://leader-node:8080/pay" {
				t.Fatalf("url = %s, want %s", req.URL.String(), "http://leader-node:8080/pay")
			}
			if got := req.Header.Get(receivedByHeader); got != "node-b" {
				t.Fatalf("forwarded %s header = %q, want %q", receivedByHeader, got, "node-b")
			}
			payloadBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read forwarded body: %v", err)
			}
			var in replication.PaymentRequest
			if err := json.Unmarshal(payloadBytes, &in); err != nil {
				t.Fatalf("decode forwarded body: %v", err)
			}
			if in.PaymentID != "pay-forwarded" {
				t.Fatalf("forwarded payment_id = %q, want %q", in.PaymentID, "pay-forwarded")
			}

			respBody, _ := json.Marshal(replication.PaymentResponse{
				Status:    "OK",
				PaymentID: "pay-forwarded",
				Message:   "processed by leader",
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader(respBody)),
			}, nil
		}),
	}

	coord := &stubCoordinator{
		role:      coordination.RoleFollower,
		leaderURL: "http://leader-node:8080",
		term:      3,
		logHead:   10,
	}

	h := NewHandler(Config{
		NodeID:           "node-b",
		CORSAllowed:      "*",
		LeaderHTTPClient: leaderClient,
	}, coord, newStubLedgerStore(), stubReplicator{})

	resp := performJSONRequest(t, h, http.MethodPost, "/pay", replication.PaymentRequest{
		PaymentID: "pay-forwarded",
		Amount:    10,
		Currency:  "USD",
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var out replication.PaymentResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if out.PaymentID != "pay-forwarded" {
		t.Fatalf("payment_id = %q, want %q", out.PaymentID, "pay-forwarded")
	}
	if out.Message != "processed by leader" {
		t.Fatalf("message = %q, want %q", out.Message, "processed by leader")
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
