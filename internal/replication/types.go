package replication

import "fmt"

// Status represents the lifecycle state of a replicated payment entry.
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusCommitted Status = "COMMITTED"
	StatusFailed    Status = "FAILED"
)

// IsValid checks whether the status is one of the supported values.
func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusCommitted, StatusFailed:
		return true
	default:
		return false
	}
}

// String returns the string representation of the status.
func (s Status) String() string {
	return string(s)
}

// LogEntry is the shared replicated ledger record.
//
// Ownership notes:
// - Member 2 owns the replicated payment/ledger shape.
// - Member 4 depends on LogIndex and Term for coordination/validation.
// - Member 3 may later use PhysicalTime and LogicalTime.
// - Member 1 may later use LogIndex for replay and catch-up.
type LogEntry struct {
	// Global ordering / coordination metadata
	LogIndex int64  `json:"log_index"`
	Term     int64  `json:"term"`
	LeaderID string `json:"leader_id"`

	// Payment payload
	PaymentID string  `json:"payment_id"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`

	// Replication / visibility state
	Status Status `json:"status"`

	// Optional timing fields for future time-sync integration
	PhysicalTime int64 `json:"physical_time"`
	LogicalTime  int64 `json:"logical_time"`
}

// Validate performs basic structural validation for the replicated entry.
// This is intentionally lightweight for now and can be extended later.
func (e LogEntry) Validate() error {
	if e.LogIndex < 0 {
		return fmt.Errorf("log_index cannot be negative")
	}
	if e.Term < 0 {
		return fmt.Errorf("term cannot be negative")
	}
	if e.PaymentID == "" {
		return fmt.Errorf("payment_id is required")
	}
	if e.Amount < 0 {
		return fmt.Errorf("amount cannot be negative")
	}
	if e.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if !e.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", e.Status)
	}
	return nil
}

// IsZero reports whether the entry is effectively empty/uninitialized.
func (e LogEntry) IsZero() bool {
	return e.LogIndex == 0 &&
		e.Term == 0 &&
		e.LeaderID == "" &&
		e.PaymentID == "" &&
		e.Amount == 0 &&
		e.Currency == "" &&
		e.Status == ""
}

// PaymentRequest is the public client request body for POST /pay.
// This is not the replicated record itself; it is the external input that
// the leader converts into a LogEntry.
type PaymentRequest struct {
	PaymentID string  `json:"payment_id"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
}

// Validate performs basic validation for an incoming payment request.
func (r PaymentRequest) Validate() error {
	if r.PaymentID == "" {
		return fmt.Errorf("payment_id is required")
	}
	if r.Amount < 0 {
		return fmt.Errorf("amount cannot be negative")
	}
	if r.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	return nil
}

// PaymentResponse is the public response body for POST /pay.
// This is shaped for frontend/API use and can be refined later when the
// endpoint is implemented.
type PaymentResponse struct {
	Status    string `json:"status"`
	PaymentID string `json:"payment_id"`

	// The leader may return these when useful for clients or debugging.
	LogIndex int64  `json:"log_index,omitempty"`
	Term     int64  `json:"term,omitempty"`
	LeaderID string `json:"leader_id,omitempty"`
	LeaderURL string `json:"leader_url,omitempty"`

	Message string `json:"message,omitempty"`
}

// ReplicationResult is a small internal summary that can be useful later
// when implementing quorum logic.
type ReplicationResult struct {
	RequiredAcks int `json:"required_acks"`
	ReceivedAcks int `json:"received_acks"`
	Committed    bool `json:"committed"`
}