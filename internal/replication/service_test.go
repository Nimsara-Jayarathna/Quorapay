package replication

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

type stubLocalLedger struct {
	appendErr    error
	commitErr    error
	cancelErr    error
	lastAppended LogEntry
	lastCommitID string
	lastCancelID string
	appendCalls  int
	commitCalls  int
	cancelCalls  int
}

func (s *stubLocalLedger) AppendPending(_ context.Context, entry LogEntry) error {
	s.appendCalls++
	s.lastAppended = entry
	return s.appendErr
}

func (s *stubLocalLedger) CommitByPaymentID(_ context.Context, paymentID string) error {
	s.commitCalls++
	s.lastCommitID = paymentID
	return s.commitErr
}

func (s *stubLocalLedger) CancelByPaymentID(_ context.Context, paymentID string) error {
	s.cancelCalls++
	s.lastCancelID = paymentID
	return s.cancelErr
}

func (s *stubLocalLedger) ExistsByPaymentID(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type stubTransport struct {
	sync.Mutex
	appendResults map[string]AppendEntriesResponse
	appendErrors  map[string]error
	commitResults map[string]CommitResponse
	commitErrors  map[string]error
	cancelResults map[string]CancelResponse
	cancelErrors  map[string]error
	appendCalls   int
	commitCalls   int
	cancelCalls   int
	lastRequests  map[string]AppendEntriesRequest
	lastCommits   map[string]CommitRequest
	lastCancels   map[string]CancelRequest
}

func (s *stubTransport) AppendToFollower(_ context.Context, followerBaseURL string, req AppendEntriesRequest) (AppendEntriesResponse, error) {
	s.Lock()
	defer s.Unlock()

	s.appendCalls++
	if s.lastRequests == nil {
		s.lastRequests = make(map[string]AppendEntriesRequest)
	}
	s.lastRequests[followerBaseURL] = req

	if err, ok := s.appendErrors[followerBaseURL]; ok {
		return AppendEntriesResponse{}, err
	}
	if resp, ok := s.appendResults[followerBaseURL]; ok {
		return resp, nil
	}

	return AppendEntriesResponse{Success: false, Message: "no response configured"}, nil
}

func (s *stubTransport) CommitToFollower(_ context.Context, followerBaseURL string, req CommitRequest) (CommitResponse, error) {
	s.Lock()
	defer s.Unlock()

	s.commitCalls++
	if s.lastCommits == nil {
		s.lastCommits = make(map[string]CommitRequest)
	}
	s.lastCommits[followerBaseURL] = req

	if err, ok := s.commitErrors[followerBaseURL]; ok {
		return CommitResponse{}, err
	}
	if resp, ok := s.commitResults[followerBaseURL]; ok {
		return resp, nil
	}

	return CommitResponse{Success: false, Message: "no commit response configured"}, nil
}

func (s *stubTransport) CancelToFollower(_ context.Context, followerBaseURL string, req CancelRequest) (CancelResponse, error) {
	s.Lock()
	defer s.Unlock()

	s.cancelCalls++
	if s.lastCancels == nil {
		s.lastCancels = make(map[string]CancelRequest)
	}
	s.lastCancels[followerBaseURL] = req

	if err, ok := s.cancelErrors[followerBaseURL]; ok {
		return CancelResponse{}, err
	}
	if resp, ok := s.cancelResults[followerBaseURL]; ok {
		return resp, nil
	}
	return CancelResponse{Success: false, Message: "no cancel response configured"}, nil
}

func baseEntry() LogEntry {
	return LogEntry{
		LogIndex:  1,
		Term:      1,
		LeaderID:  "A",
		PaymentID: "pay-1",
		Amount:    10,
		Currency:  "USD",
		Status:    StatusPending,
	}
}

func TestReplicationService_ReplicateWithQuorum_LocalAppendInitializesAckCount(t *testing.T) {
	ledger := &stubLocalLedger{}
	svc := NewReplicationService(ledger, &stubTransport{})

	result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), nil)
	if err != nil {
		t.Fatalf("ReplicateWithQuorum() error = %v", err)
	}

	if !result.LocalAppendOK {
		t.Fatalf("LocalAppendOK = false, want true")
	}
	if result.AckCount != 1 {
		t.Fatalf("AckCount = %d, want 1", result.AckCount)
	}
	if ledger.appendCalls != 1 {
		t.Fatalf("AppendPending calls = %d, want 1", ledger.appendCalls)
	}
	if ledger.lastAppended.Status != StatusPending {
		t.Fatalf("appended status = %q, want %q", ledger.lastAppended.Status, StatusPending)
	}
	if ledger.commitCalls != 1 {
		t.Fatalf("CommitByPaymentID calls = %d, want 1", ledger.commitCalls)
	}
}

func TestReplicationService_ReplicateWithQuorum_RequiredQuorumCalculation(t *testing.T) {
	tests := []struct {
		name         string
		followers    []string
		wantRequired int
		wantReached  bool
	}{
		{
			name:         "single node",
			followers:    nil,
			wantRequired: 1,
			wantReached:  true,
		},
		{
			name:         "three nodes",
			followers:    []string{"http://node-b:8002", "http://node-c:8003"},
			wantRequired: 2,
			wantReached:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger := &stubLocalLedger{}
			svc := NewReplicationService(ledger, &stubTransport{})

			result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), tt.followers)
			if err != nil {
				t.Fatalf("ReplicateWithQuorum() error = %v", err)
			}

			if result.RequiredQuorum != tt.wantRequired {
				t.Fatalf("RequiredQuorum = %d, want %d", result.RequiredQuorum, tt.wantRequired)
			}
			if result.QuorumReached != tt.wantReached {
				t.Fatalf("QuorumReached = %v, want %v", result.QuorumReached, tt.wantReached)
			}
			if result.AckCount != 1 {
				t.Fatalf("AckCount = %d, want 1", result.AckCount)
			}
		})
	}
}

func TestReplicationService_ReplicateWithQuorum_LocalAppendFailure(t *testing.T) {
	appendErr := errors.New("db write failed")
	ledger := &stubLocalLedger{appendErr: appendErr}
	svc := NewReplicationService(ledger, &stubTransport{})

	result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), nil)
	if err == nil {
		t.Fatalf("ReplicateWithQuorum() expected error")
	}
	if !errors.Is(err, appendErr) {
		t.Fatalf("error = %v, want wrapped append error", err)
	}

	if result.LocalAppendOK {
		t.Fatalf("LocalAppendOK = true, want false")
	}
	if result.AckCount != 0 {
		t.Fatalf("AckCount = %d, want 0", result.AckCount)
	}
	if result.RequiredQuorum != 1 {
		t.Fatalf("RequiredQuorum = %d, want 1", result.RequiredQuorum)
	}
}

func TestReplicationService_ReplicateWithQuorum_OneFollowerSuccessReachesQuorum(t *testing.T) {
	ledger := &stubLocalLedger{}
	transport := &stubTransport{
		appendResults: map[string]AppendEntriesResponse{
			"http://node-b:8002": {Success: true},
			"http://node-c:8003": {Success: false, Message: "reject"},
		},
	}
	svc := NewReplicationService(ledger, transport)

	followers := []string{"http://node-b:8002", "http://node-c:8003"}
	result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), followers)
	if err != nil {
		t.Fatalf("ReplicateWithQuorum() error = %v", err)
	}

	if result.RequiredQuorum != 2 {
		t.Fatalf("RequiredQuorum = %d, want 2", result.RequiredQuorum)
	}
	if result.AckCount != 2 {
		t.Fatalf("AckCount = %d, want 2", result.AckCount)
	}
	if !result.QuorumReached {
		t.Fatalf("QuorumReached = false, want true")
	}
	if !result.FollowerResults[0].AppendAcknowledged {
		t.Fatalf("follower 0 append ack = false, want true")
	}
}

func TestReplicationService_ReplicateWithQuorum_BothFollowersFailNoQuorum(t *testing.T) {
	ledger := &stubLocalLedger{}
	transport := &stubTransport{
		appendErrors: map[string]error{
			"http://node-b:8002": errors.New("timeout"),
			"http://node-c:8003": errors.New("connection refused"),
		},
	}
	svc := NewReplicationService(ledger, transport)

	followers := []string{"http://node-b:8002", "http://node-c:8003"}
	result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), followers)
	if err != nil {
		t.Fatalf("ReplicateWithQuorum() error = %v", err)
	}

	if result.AckCount != 1 {
		t.Fatalf("AckCount = %d, want 1", result.AckCount)
	}
	if result.QuorumReached {
		t.Fatalf("QuorumReached = true, want false")
	}
	if result.FollowerResults[0].Error == "" || result.FollowerResults[1].Error == "" {
		t.Fatalf("expected follower errors to be recorded")
	}
	if ledger.commitCalls != 0 {
		t.Fatalf("CommitByPaymentID calls = %d, want 0", ledger.commitCalls)
	}
	if transport.commitCalls != 0 {
		t.Fatalf("CommitToFollower calls = %d, want 0", transport.commitCalls)
	}
}

func TestReplicationService_ReplicateWithQuorum_MixedFollowerOutcomesCountAcks(t *testing.T) {
	ledger := &stubLocalLedger{}
	transport := &stubTransport{
		appendResults: map[string]AppendEntriesResponse{
			"http://node-b:8002": {Success: true},
			"http://node-c:8003": {Success: false, Message: "stale term"},
			"http://node-d:8004": {Success: true},
		},
		commitResults: map[string]CommitResponse{
			"http://node-b:8002": {Success: true},
			"http://node-c:8003": {Success: true},
			"http://node-d:8004": {Success: true},
		},
	}
	svc := NewReplicationService(ledger, transport)

	followers := []string{"http://node-b:8002", "http://node-c:8003", "http://node-d:8004"}
	result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), followers)
	if err != nil {
		t.Fatalf("ReplicateWithQuorum() error = %v", err)
	}

	if result.RequiredQuorum != 3 {
		t.Fatalf("RequiredQuorum = %d, want 3", result.RequiredQuorum)
	}
	if result.AckCount != 3 {
		t.Fatalf("AckCount = %d, want 3", result.AckCount)
	}
	if !result.QuorumReached {
		t.Fatalf("QuorumReached = false, want true")
	}
	if result.FollowerResults[1].Error != "stale term" {
		t.Fatalf("follower error = %q, want stale term", result.FollowerResults[1].Error)
	}

	for _, follower := range followers {
		req, ok := transport.lastRequests[follower]
		if !ok {
			t.Fatalf("missing append request for follower %s", follower)
		}
		if len(req.Entries) != 1 {
			t.Fatalf("entries len for %s = %d, want 1", follower, len(req.Entries))
		}
		if req.Entries[0].Status != StatusPending {
			t.Fatalf("entry status for %s = %q, want %q", follower, req.Entries[0].Status, StatusPending)
		}
		if req.LeaderID == "" {
			t.Fatalf("leader_id for %s is empty", follower)
		}
	}

	if transport.appendCalls != len(followers) {
		t.Fatalf("append calls = %d, want %d", transport.appendCalls, len(followers))
	}
	if ledger.commitCalls != 1 {
		t.Fatalf("CommitByPaymentID calls = %d, want 1", ledger.commitCalls)
	}
}

func TestReplicationService_ReplicateWithQuorum_QuorumReachedCommitsLocally(t *testing.T) {
	ledger := &stubLocalLedger{}
	transport := &stubTransport{
		appendResults: map[string]AppendEntriesResponse{
			"http://node-b:8002": {Success: true},
			"http://node-c:8003": {Success: false, Message: "reject"},
		},
		commitResults: map[string]CommitResponse{
			"http://node-b:8002": {Success: true},
			"http://node-c:8003": {Success: true},
		},
	}
	svc := NewReplicationService(ledger, transport)

	result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), []string{"http://node-b:8002", "http://node-c:8003"})
	if err != nil {
		t.Fatalf("ReplicateWithQuorum() error = %v", err)
	}

	if !result.QuorumReached {
		t.Fatalf("QuorumReached = false, want true")
	}
	if !result.Committed {
		t.Fatalf("Committed = false, want true")
	}
	if ledger.commitCalls != 1 {
		t.Fatalf("CommitByPaymentID calls = %d, want 1", ledger.commitCalls)
	}
	if ledger.lastCommitID != "pay-1" {
		t.Fatalf("last commit payment id = %q, want %q", ledger.lastCommitID, "pay-1")
	}
}

func TestReplicationService_ReplicateWithQuorum_QuorumNotReachedSkipsLocalCommit(t *testing.T) {
	ledger := &stubLocalLedger{}
	transport := &stubTransport{
		appendErrors: map[string]error{
			"http://node-b:8002": errors.New("down"),
			"http://node-c:8003": errors.New("down"),
		},
	}
	svc := NewReplicationService(ledger, transport)

	result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), []string{"http://node-b:8002", "http://node-c:8003"})
	if err != nil {
		t.Fatalf("ReplicateWithQuorum() error = %v", err)
	}

	if result.QuorumReached {
		t.Fatalf("QuorumReached = true, want false")
	}
	if result.Committed {
		t.Fatalf("Committed = true, want false")
	}
	if ledger.commitCalls != 0 {
		t.Fatalf("CommitByPaymentID calls = %d, want 0", ledger.commitCalls)
	}
	if transport.commitCalls != 0 {
		t.Fatalf("CommitToFollower calls = %d, want 0", transport.commitCalls)
	}
}

func TestReplicationService_ReplicateWithQuorum_LocalCommitFailureAfterQuorumReturnsError(t *testing.T) {
	commitErr := errors.New("commit failed")
	ledger := &stubLocalLedger{commitErr: commitErr}
	transport := &stubTransport{
		appendResults: map[string]AppendEntriesResponse{
			"http://node-b:8002": {Success: true},
			"http://node-c:8003": {Success: false, Message: "reject"},
		},
	}
	svc := NewReplicationService(ledger, transport)

	result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), []string{"http://node-b:8002", "http://node-c:8003"})
	if err == nil {
		t.Fatalf("ReplicateWithQuorum() expected local commit error")
	}
	if !errors.Is(err, commitErr) {
		t.Fatalf("error = %v, want wrapped commit error", err)
	}

	if !result.QuorumReached {
		t.Fatalf("QuorumReached = false, want true")
	}
	if result.Committed {
		t.Fatalf("Committed = true, want false")
	}
	if result.AckCount != 2 {
		t.Fatalf("AckCount = %d, want 2", result.AckCount)
	}
	if ledger.commitCalls != 1 {
		t.Fatalf("CommitByPaymentID calls = %d, want 1", ledger.commitCalls)
	}
}

func TestReplicationService_ReplicateWithQuorum_QuorumReachedSendsFollowerCommits(t *testing.T) {
	ledger := &stubLocalLedger{}
	transport := &stubTransport{
		appendResults: map[string]AppendEntriesResponse{
			"http://node-b:8002": {Success: true},
			"http://node-c:8003": {Success: false, Message: "append rejected"},
		},
		commitResults: map[string]CommitResponse{
			"http://node-b:8002": {Success: true},
			"http://node-c:8003": {Success: true},
		},
	}
	svc := NewReplicationService(ledger, transport)
	followers := []string{"http://node-b:8002", "http://node-c:8003"}

	result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), followers)
	if err != nil {
		t.Fatalf("ReplicateWithQuorum() error = %v", err)
	}

	if !result.QuorumReached {
		t.Fatalf("QuorumReached = false, want true")
	}
	if !result.Committed {
		t.Fatalf("Committed = false, want true")
	}
	if transport.commitCalls != len(followers) {
		t.Fatalf("CommitToFollower calls = %d, want %d", transport.commitCalls, len(followers))
	}
	for _, follower := range followers {
		req, ok := transport.lastCommits[follower]
		if !ok {
			t.Fatalf("missing commit request for follower %s", follower)
		}
		if req.PaymentID != "pay-1" {
			t.Fatalf("commit payment id for %s = %q, want %q", follower, req.PaymentID, "pay-1")
		}
		if req.LogIndex != 1 {
			t.Fatalf("commit log index for %s = %d, want 1", follower, req.LogIndex)
		}
	}
	if !result.FollowerResults[0].CommitAcknowledged || !result.FollowerResults[1].CommitAcknowledged {
		t.Fatalf("expected follower commit acknowledgements to be true")
	}
}

func TestReplicationService_ReplicateWithQuorum_QuorumNotReachedSendsNoFollowerCommits(t *testing.T) {
	ledger := &stubLocalLedger{}
	transport := &stubTransport{
		appendErrors: map[string]error{
			"http://node-b:8002": errors.New("down"),
			"http://node-c:8003": errors.New("down"),
		},
	}
	svc := NewReplicationService(ledger, transport)

	result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), []string{"http://node-b:8002", "http://node-c:8003"})
	if err != nil {
		t.Fatalf("ReplicateWithQuorum() error = %v", err)
	}

	if result.QuorumReached {
		t.Fatalf("QuorumReached = true, want false")
	}
	if transport.commitCalls != 0 {
		t.Fatalf("CommitToFollower calls = %d, want 0", transport.commitCalls)
	}
}

func TestReplicationService_ReplicateWithQuorum_FollowerCommitFailureRecordedNonFatal(t *testing.T) {
	ledger := &stubLocalLedger{}
	transport := &stubTransport{
		appendResults: map[string]AppendEntriesResponse{
			"http://node-b:8002": {Success: true},
			"http://node-c:8003": {Success: false, Message: "append rejected"},
		},
		commitResults: map[string]CommitResponse{
			"http://node-b:8002": {Success: true},
		},
		commitErrors: map[string]error{
			"http://node-c:8003": errors.New("commit timeout"),
		},
	}
	svc := NewReplicationService(ledger, transport)

	result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), []string{"http://node-b:8002", "http://node-c:8003"})
	if err != nil {
		t.Fatalf("ReplicateWithQuorum() error = %v", err)
	}

	if !result.QuorumReached || !result.Committed {
		t.Fatalf("expected quorum and local commit success")
	}
	if transport.commitCalls != 2 {
		t.Fatalf("CommitToFollower calls = %d, want 2", transport.commitCalls)
	}
	if !result.FollowerResults[0].CommitAcknowledged {
		t.Fatalf("follower 0 commit ack = false, want true")
	}
	if result.FollowerResults[1].Error == "" || !strings.Contains(result.FollowerResults[1].Error, "commit") {
		t.Fatalf("follower 1 error = %q, expected commit error context", result.FollowerResults[1].Error)
	}
}

func TestReplicationService_ReplicateWithQuorum_TransportRequiredWhenFollowersPresent(t *testing.T) {
	ledger := &stubLocalLedger{}
	svc := NewReplicationService(ledger, nil)

	_, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), []string{"http://node-b:8002"})
	if err == nil {
		t.Fatalf("ReplicateWithQuorum() expected transport configuration error")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "transport") {
		t.Fatalf("error = %q, expected transport context", got)
	}
}
