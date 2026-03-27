package replication

import (
	"context"
	"errors"
	"testing"
)

type stubLocalLedger struct {
	appendErr    error
	lastAppended LogEntry
	appendCalls  int
}

func (s *stubLocalLedger) AppendPending(_ context.Context, entry LogEntry) error {
	s.appendCalls++
	s.lastAppended = entry
	return s.appendErr
}

func (s *stubLocalLedger) CommitByPaymentID(context.Context, string) error {
	return nil
}

type panicTransport struct{}

func (panicTransport) AppendToFollower(context.Context, string, AppendEntriesRequest) (AppendEntriesResponse, error) {
	panic("AppendToFollower should not be called in phase 4.2")
}

func (panicTransport) CommitToFollower(context.Context, string, CommitRequest) (CommitResponse, error) {
	panic("CommitToFollower should not be called in phase 4.2")
}

func baseEntry() LogEntry {
	return LogEntry{
		LogIndex:  1,
		Term:      1,
		LeaderID:  "A",
		PaymentID: "pay-1",
		Amount:    10,
		Currency:  "USD",
		Status:    StatusCommitted,
	}
}

func TestReplicationService_ReplicateWithQuorum_LocalAppendInitializesAckCount(t *testing.T) {
	ledger := &stubLocalLedger{}
	svc := NewReplicationService(ledger, panicTransport{})

	result, err := svc.ReplicateWithQuorum(context.Background(), baseEntry(), []string{"http://node-b:8002"})
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
			svc := NewReplicationService(ledger, panicTransport{})

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
	svc := NewReplicationService(ledger, panicTransport{})

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
