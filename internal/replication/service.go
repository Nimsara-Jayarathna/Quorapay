package replication

import (
	"context"
	"errors"
)

var ErrQuorumReplicationNotImplemented = errors.New("quorum replication is not implemented yet")

// LocalLedger defines the local persistence operations needed by quorum replication.
type LocalLedger interface {
	AppendPending(context.Context, LogEntry) error
	CommitByPaymentID(context.Context, string) error
}

// FollowerTransport defines leader-to-follower replication transport operations.
type FollowerTransport interface {
	AppendToFollower(context.Context, string, AppendEntriesRequest) (AppendEntriesResponse, error)
	CommitToFollower(context.Context, string, CommitRequest) (CommitResponse, error)
}

// FollowerReplicationResult records per-follower replication outcome details.
type FollowerReplicationResult struct {
	FollowerBaseURL    string `json:"follower_base_url"`
	AppendAcknowledged bool   `json:"append_acknowledged"`
	CommitAcknowledged bool   `json:"commit_acknowledged"`
	Error              string `json:"error,omitempty"`
}

// QuorumReplicationResult is the leader-side summary returned by quorum replication.
type QuorumReplicationResult struct {
	PaymentID       string                      `json:"payment_id"`
	QuorumReached   bool                        `json:"quorum_reached"`
	LocalAppendOK   bool                        `json:"local_append_ok"`
	AckCount        int                         `json:"ack_count"`
	RequiredQuorum  int                         `json:"required_quorum"`
	Committed       bool                        `json:"committed"`
	FollowerResults []FollowerReplicationResult `json:"follower_results,omitempty"`
}

// ReplicationService orchestrates future quorum replication steps from the leader.
type ReplicationService struct {
	ledger    LocalLedger
	transport FollowerTransport
}

func NewReplicationService(ledger LocalLedger, transport FollowerTransport) *ReplicationService {
	return &ReplicationService{
		ledger:    ledger,
		transport: transport,
	}
}

// ReplicateWithQuorum is a placeholder for the future end-to-end quorum flow.
func (s *ReplicationService) ReplicateWithQuorum(ctx context.Context, entry LogEntry, followerBaseURLs []string) (QuorumReplicationResult, error) {
	_ = ctx
	_ = s.ledger
	_ = s.transport

	result := QuorumReplicationResult{
		PaymentID:       entry.PaymentID,
		QuorumReached:   false,
		LocalAppendOK:   false,
		AckCount:        0,
		RequiredQuorum:  0,
		Committed:       false,
		FollowerResults: make([]FollowerReplicationResult, 0, len(followerBaseURLs)),
	}

	for _, baseURL := range followerBaseURLs {
		result.FollowerResults = append(result.FollowerResults, FollowerReplicationResult{FollowerBaseURL: baseURL})
	}

	return result, ErrQuorumReplicationNotImplemented
}
