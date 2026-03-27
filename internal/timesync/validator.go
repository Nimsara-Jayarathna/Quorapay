package timesync

import (
    "fmt"
    "time"
)

// MaxMessageAge is the maximum age of an incoming message timestamp before it is rejected.
const MaxMessageAge = 2 * time.Second

// MaxFutureAllowance is how far into the future a timestamp can be before it is rejected.
const MaxFutureAllowance = 500 * time.Millisecond

// Validator checks whether an incoming transaction timestamp is acceptable.
type Validator struct {
    skew *SkewTracker
}

// NewValidator creates a new Validator backed by a SkewTracker.
func NewValidator(skew *SkewTracker) *Validator {
    return &Validator{skew: skew}
}

// Validate checks if the incoming wall-clock timestamp from a node is acceptable.
// Returns nil if valid, or an error describing why it was rejected.
func (v *Validator) Validate(nodeID string, msgTime time.Time) error {
    now := time.Now()

    // Check if the message is too old (delayed message)
    age := now.Sub(msgTime)
    if age > MaxMessageAge {
        return fmt.Errorf("message from %s is too old: age=%v exceeds max=%v", nodeID, age, MaxMessageAge)
    }

    // Check if the message is too far in the future (clock running ahead)
    if msgTime.Sub(now) > MaxFutureAllowance {
        return fmt.Errorf("message from %s is too far in the future: drift=%v exceeds max=%v", nodeID, msgTime.Sub(now), MaxFutureAllowance)
    }

    // Check skew tolerance if we have data for this node
    if ok, err := v.skew.IsWithinTolerance(nodeID); err == nil && !ok {
        return fmt.Errorf("message from %s rejected: clock skew exceeds tolerance", nodeID)
    }

    return nil
}