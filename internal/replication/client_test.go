package replication

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPClient_AppendToFollowerSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/internal/append" {
			t.Fatalf("path = %s, want /internal/append", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content-type = %s, want application/json", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"term":2,"last_log_index":7,"message":"ok"}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.Client())
	resp, err := client.AppendToFollower(context.Background(), srv.URL, AppendEntriesRequest{
		LeaderID: "A",
		Term:     2,
		Entries: []LogEntry{{
			LogIndex:  7,
			Term:      2,
			LeaderID:  "A",
			PaymentID: "pay-1",
			Amount:    10,
			Currency:  "USD",
			Status:    StatusPending,
		}},
	})
	if err != nil {
		t.Fatalf("AppendToFollower() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("resp.Success = false, want true")
	}
	if resp.LastLogIndex != 7 {
		t.Fatalf("resp.LastLogIndex = %d, want 7", resp.LastLogIndex)
	}
}

func TestHTTPClient_CommitToFollowerSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/commit" {
			t.Fatalf("path = %s, want /internal/commit", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"committed"}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.Client())
	resp, err := client.CommitToFollower(context.Background(), srv.URL, CommitRequest{PaymentID: "pay-1", LogIndex: 7})
	if err != nil {
		t.Fatalf("CommitToFollower() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("resp.Success = false, want true")
	}
}

func TestHTTPClient_FollowerNon2xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"db failed"}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.Client())
	_, err := client.AppendToFollower(context.Background(), srv.URL, AppendEntriesRequest{
		LeaderID: "A",
		Term:     2,
		Entries:  []LogEntry{{LogIndex: 1, Term: 2, LeaderID: "A", PaymentID: "pay-1", Amount: 1, Currency: "USD", Status: StatusPending}},
	})
	if err == nil {
		t.Fatalf("AppendToFollower() expected error")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("error = %q, want status code context", err)
	}
	if !strings.Contains(err.Error(), "db failed") {
		t.Fatalf("error = %q, want follower message", err)
	}
}

func TestHTTPClient_InvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.Client())
	_, err := client.CommitToFollower(context.Background(), srv.URL, CommitRequest{PaymentID: "pay-1", LogIndex: 2})
	if err == nil {
		t.Fatalf("CommitToFollower() expected decode error")
	}
	if !strings.Contains(err.Error(), "decode response JSON") {
		t.Fatalf("error = %q, want decode context", err)
	}
}

func TestHTTPClient_ContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"message":"ok"}`))
	}))
	defer srv.Close()

	httpClient := srv.Client()
	httpClient.Timeout = 0
	client := NewHTTPClient(httpClient)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.AppendToFollower(ctx, srv.URL, AppendEntriesRequest{
		LeaderID: "A",
		Term:     1,
		Entries:  []LogEntry{{LogIndex: 1, Term: 1, LeaderID: "A", PaymentID: "pay-timeout", Amount: 1, Currency: "USD", Status: StatusPending}},
	})
	if err == nil {
		t.Fatalf("AppendToFollower() expected timeout/cancel error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(strings.ToLower(err.Error()), "deadline") {
		t.Fatalf("error = %q, want context deadline exceeded", err)
	}
}

func TestHTTPClient_CatchUpFromLeaderSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/internal/catchup" {
			t.Fatalf("path = %s, want /internal/catchup", r.URL.Path)
		}
		if got := r.URL.Query().Get("from_log_index"); got != "7" {
			t.Fatalf("from_log_index = %s, want 7", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"entries":[{"log_index":8,"term":2,"leader_id":"C","payment_id":"pay-8","amount":10,"currency":"USD","status":"COMMITTED"}]}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.Client())
	resp, err := client.CatchUpFromLeader(context.Background(), srv.URL, 7)
	if err != nil {
		t.Fatalf("CatchUpFromLeader() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("resp.Success = false, want true")
	}
	if len(resp.Entries) != 1 || resp.Entries[0].PaymentID != "pay-8" {
		t.Fatalf("entries = %+v, want single pay-8", resp.Entries)
	}
}
