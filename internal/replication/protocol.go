package replication

import "fmt"

type AppendEntriesRequest struct {
	LeaderID string     `json:"leader_id"`
	Term     int64      `json:"term"`
	Entries  []LogEntry `json:"entries"`
}

// Validate performs basic structural validation for an append request.
func (r AppendEntriesRequest) Validate() error {
	if r.LeaderID == "" {
		return fmt.Errorf("leader_id is required")
	}
	if r.Term < 0 {
		return fmt.Errorf("term cannot be negative")
	}
	if len(r.Entries) == 0 {
		return fmt.Errorf("entries cannot be empty")
	}

	for i, entry := range r.Entries {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("entries[%d] invalid: %w", i, err)
		}
	}

	return nil
}

// AppendEntriesResponse is returned by a follower after receiving an append
// request from the leader.
//
// Success indicates whether the append was accepted.
// Term allows the leader to notice if the follower is already on a newer term.
// LastLogIndex helps the leader understand the follower's current replicated state.
// Message provides optional debugging/context information.
type AppendEntriesResponse struct {
	Success      bool   `json:"success"`
	Term         int64  `json:"term"`
	LastLogIndex int64  `json:"last_log_index"`
	Message      string `json:"message,omitempty"`
}

// Validate performs basic structural validation for an append response.
func (r AppendEntriesResponse) Validate() error {
	if r.Term < 0 {
		return fmt.Errorf("term cannot be negative")
	}
	if r.LastLogIndex < 0 {
		return fmt.Errorf("last_log_index cannot be negative")
	}
	return nil
}

// CommitRequest is sent by the leader to followers after quorum is reached,
// instructing them to transition a replicated entry to COMMITTED state.
//
// For now, LogIndex is enough because the log index uniquely identifies the
// target entry in the replicated ledger.
type CommitRequest struct {
	LogIndex  int64  `json:"log_index"`
	PaymentID string `json:"payment_id"`
}

// Validate performs basic structural validation for a commit request.
func (r CommitRequest) Validate() error {
	if r.LogIndex < 0 {
		return fmt.Errorf("log_index cannot be negative")
	}
	return nil
}

// CommitResponse is returned by a follower after processing a commit request.
type CommitResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// CatchUpRequest is reserved for recovery/rejoin scenarios, where a node asks
// for entries after a known log index.
//
// This is useful for Member 1 later and keeping it here early avoids breaking
// protocol changes later.
type CatchUpRequest struct {
	FromLogIndex int64 `json:"from_log_index"`
}

// Validate performs basic structural validation for a catch-up request.
func (r CatchUpRequest) Validate() error {
	if r.FromLogIndex < 0 {
		return fmt.Errorf("from_log_index cannot be negative")
	}
	return nil
}

// CatchUpResponse returns replicated entries starting after the requested index.
type CatchUpResponse struct {
	Success bool       `json:"success"`
	Entries []LogEntry `json:"entries,omitempty"`
	Message string     `json:"message,omitempty"`
}
