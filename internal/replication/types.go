package replication

import "fmt"

// Status is the lifecycle state of a replicated entry.
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusCommitted Status = "COMMITTED"
	StatusFailed    Status = "FAILED"
)

// IsValid reports whether the status is supported.
func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusCommitted, StatusFailed:
		return true
	default:
		return false
	}
}

// String returns the status as text.
func (s Status) String() string {
	return string(s)
}

// LogEntry is the replicated ledger record.
type LogEntry struct {
	// Ordering metadata.
	LogIndex int64  `json:"log_index"`
	Term     int64  `json:"term"`
	LeaderID string `json:"leader_id"`

	// Payment payload.
	PaymentID string  `json:"payment_id"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`

	// Replication state.
	Status Status `json:"status"`

	// Optional time-sync fields.
	PhysicalTime int64 `json:"physical_time"`
	LogicalTime  int64 `json:"logical_time"`
}

// Validate checks entry shape.
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

// IsZero reports whether the entry is empty.
func (e LogEntry) IsZero() bool {
	return e.LogIndex == 0 &&
		e.Term == 0 &&
		e.LeaderID == "" &&
		e.PaymentID == "" &&
		e.Amount == 0 &&
		e.Currency == "" &&
		e.Status == ""
}

// PaymentRequest is the public input for POST /pay.
type PaymentRequest struct {
	PaymentID string  `json:"payment_id"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
}

// Validate checks payment request shape.
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

// PaymentResponse is the public output for POST /pay.
type PaymentResponse struct {
	Status    string `json:"status"`
	PaymentID string `json:"payment_id"`

	// Optional leader metadata.
	LogIndex  int64  `json:"log_index,omitempty"`
	Term      int64  `json:"term,omitempty"`
	LeaderID  string `json:"leader_id,omitempty"`
	LeaderURL string `json:"leader_url,omitempty"`

	Message string `json:"message,omitempty"`
}

// ReplicationResult is a small quorum summary.
type ReplicationResult struct {
	RequiredAcks int  `json:"required_acks"`
	ReceivedAcks int  `json:"received_acks"`
	Committed    bool `json:"committed"`
}
