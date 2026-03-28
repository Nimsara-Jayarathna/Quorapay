package replication

import "fmt"

type AppendEntriesRequest struct {
	LeaderID string     `json:"leader_id"`
	Term     int64      `json:"term"`
	Entries  []LogEntry `json:"entries"`
}

// Validate checks append request shape.
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

// AppendEntriesResponse is a follower ACK for append.
type AppendEntriesResponse struct {
	Success      bool   `json:"success"`
	Term         int64  `json:"term"`
	LastLogIndex int64  `json:"last_log_index"`
	Message      string `json:"message,omitempty"`
}

// Validate checks append response shape.
func (r AppendEntriesResponse) Validate() error {
	if r.Term < 0 {
		return fmt.Errorf("term cannot be negative")
	}
	if r.LastLogIndex < 0 {
		return fmt.Errorf("last_log_index cannot be negative")
	}
	return nil
}

// CommitRequest asks followers to mark an entry committed.
type CommitRequest struct {
	LogIndex  int64  `json:"log_index"`
	PaymentID string `json:"payment_id"`
}

// Validate checks commit request shape.
func (r CommitRequest) Validate() error {
	if r.LogIndex < 0 {
		return fmt.Errorf("log_index cannot be negative")
	}
	if r.PaymentID == "" {
		return fmt.Errorf("payment_id is required")
	}
	return nil
}

// CommitResponse is a follower ACK for commit.
type CommitResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// CatchUpRequest asks for entries after a known log index.
type CatchUpRequest struct {
	FromLogIndex int64 `json:"from_log_index"`
}

// Validate checks catch-up request shape.
func (r CatchUpRequest) Validate() error {
	if r.FromLogIndex < 0 {
		return fmt.Errorf("from_log_index cannot be negative")
	}
	return nil
}

// CatchUpResponse returns entries for a catch-up request.
type CatchUpResponse struct {
	Success        bool       `json:"success"`
	Entries        []LogEntry `json:"entries,omitempty"`
	Message        string     `json:"message,omitempty"`
	HasMore        bool       `json:"has_more"`
	TotalAvailable int        `json:"total_available"`
}
