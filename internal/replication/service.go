package replication

import (
	"context"
	"fmt"
	"sync"
)

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

// ReplicateWithQuorum applies quorum replication for a single payment log entry.
func (s *ReplicationService) ReplicateWithQuorum(ctx context.Context, entry LogEntry, followerBaseURLs []string) (QuorumReplicationResult, error) {
	if s.ledger == nil {
		return QuorumReplicationResult{}, fmt.Errorf("local ledger is not configured")
	}
	if len(followerBaseURLs) > 0 && s.transport == nil {
		return QuorumReplicationResult{}, fmt.Errorf("follower transport is not configured")
	}

	if err := entry.Validate(); err != nil {
		return QuorumReplicationResult{}, fmt.Errorf("invalid replication entry: %w", err)
	}

	entry.Status = StatusPending

	totalNodes := 1 + len(followerBaseURLs)
	requiredQuorum := totalNodes/2 + 1

	result := QuorumReplicationResult{
		PaymentID:       entry.PaymentID,
		QuorumReached:   false,
		LocalAppendOK:   false,
		AckCount:        0,
		RequiredQuorum:  requiredQuorum,
		Committed:       false,
		FollowerResults: make([]FollowerReplicationResult, 0, len(followerBaseURLs)),
	}

	for _, baseURL := range followerBaseURLs {
		result.FollowerResults = append(result.FollowerResults, FollowerReplicationResult{FollowerBaseURL: baseURL})
	}

	if err := s.ledger.AppendPending(ctx, entry); err != nil {
		return result, fmt.Errorf("append pending payment %s locally: %w", entry.PaymentID, err)
	}

	appendReq := AppendEntriesRequest{
		LeaderID: entry.LeaderID,
		Term:     entry.Term,
		Entries:  []LogEntry{entry},
	}
	if err := appendReq.Validate(); err != nil {
		return result, fmt.Errorf("build append request for payment %s: %w", entry.PaymentID, err)
	}

	result.LocalAppendOK = true
	result.AckCount = 1

	type appendResult struct {
		index  int
		ack    bool
		errMsg string
	}

	appendResults := make(chan appendResult, len(followerBaseURLs))
	var wg sync.WaitGroup

	for i, baseURL := range followerBaseURLs {
		wg.Add(1)
		go func(idx int, url string) {
			defer wg.Done()
			resp, err := s.transport.AppendToFollower(ctx, url, appendReq)
			if err != nil {
				appendResults <- appendResult{index: idx, ack: false, errMsg: err.Error()}
				return
			}
			if resp.Success {
				appendResults <- appendResult{index: idx, ack: true}
				return
			}
			msg := resp.Message
			if msg == "" {
				msg = "append not acknowledged"
			}
			appendResults <- appendResult{index: idx, ack: false, errMsg: msg}
		}(i, baseURL)
	}

	wg.Wait()
	close(appendResults)

	for ar := range appendResults {
		if ar.ack {
			result.FollowerResults[ar.index].AppendAcknowledged = true
			result.AckCount++
		} else {
			result.FollowerResults[ar.index].Error = ar.errMsg
		}
	}

	result.QuorumReached = result.AckCount >= result.RequiredQuorum
	if !result.QuorumReached {
		return result, nil
	}

	if err := s.ledger.CommitByPaymentID(ctx, entry.PaymentID); err != nil {
		return result, fmt.Errorf("commit payment %s locally after quorum: %w", entry.PaymentID, err)
	}

	result.Committed = true
	commitReq := CommitRequest{
		LogIndex:  entry.LogIndex,
		PaymentID: entry.PaymentID,
	}

	for i, baseURL := range followerBaseURLs {
		resp, err := s.transport.CommitToFollower(ctx, baseURL, commitReq)
		if err != nil {
			result.FollowerResults[i].Error = mergeFollowerError(result.FollowerResults[i].Error, fmt.Sprintf("commit: %s", err.Error()))
			continue
		}

		if resp.Success {
			result.FollowerResults[i].CommitAcknowledged = true
			continue
		}

		message := resp.Message
		if message == "" {
			message = "commit not acknowledged"
		}
		result.FollowerResults[i].Error = mergeFollowerError(result.FollowerResults[i].Error, "commit: "+message)
	}

	return result, nil
}

func mergeFollowerError(existing string, next string) string {
	if existing == "" {
		return next
	}
	if next == "" {
		return existing
	}
	return existing + " | " + next
}
