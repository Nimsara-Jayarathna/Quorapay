package coordination

import "testing"

func TestCanTransitionFaultState(t *testing.T) {
	tests := []struct {
		name    string
		current string
		next    string
		want    bool
	}{
		{name: "initial to recovering", current: "", next: FaultStateRecovering, want: true},
		{name: "recovering to rejoined", current: FaultStateRecovering, next: FaultStateRejoined, want: true},
		{name: "rejoined to healthy", current: FaultStateRejoined, next: FaultStateHealthy, want: true},
		{name: "healthy to failed", current: FaultStateHealthy, next: FaultStateFailed, want: true},
		{name: "failed to recovering", current: FaultStateFailed, next: FaultStateRecovering, want: true},
		{name: "failed to healthy invalid", current: FaultStateFailed, next: FaultStateHealthy, want: false},
		{name: "recovering to healthy invalid", current: FaultStateRecovering, next: FaultStateHealthy, want: false},
		{name: "healthy to rejoined invalid", current: FaultStateHealthy, next: FaultStateRejoined, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canTransitionFaultState(tt.current, tt.next)
			if got != tt.want {
				t.Fatalf("canTransitionFaultState(%q, %q) = %v, want %v", tt.current, tt.next, got, tt.want)
			}
		})
	}
}

func TestSetFaultState_ValidFlow(t *testing.T) {
	m := NewManager(Config{
		NodeID:      "A",
		ZKAddr:      "localhost:2181",
		StoragePath: "./data/nodeA/ledger.db",
	})

	// NewManager starts in RECOVERING.
	if got := m.status.FaultState; got != FaultStateRecovering {
		t.Fatalf("initial fault state = %q, want %q", got, FaultStateRecovering)
	}
	if !m.status.RecoveryInProgress {
		t.Fatalf("initial recovery_in_progress = false, want true")
	}

	if err := m.setFaultState(FaultStateRejoined, "node rejoined cluster"); err != nil {
		t.Fatalf("setFaultState REJOINED failed: %v", err)
	}
	if got := m.status.FaultState; got != FaultStateRejoined {
		t.Fatalf("fault state after REJOINED = %q, want %q", got, FaultStateRejoined)
	}
	if m.status.LastStateChange == "" {
		t.Fatalf("last_state_change should be set on transition")
	}
	if m.rejoinedSince.IsZero() {
		t.Fatalf("rejoinedSince should be set when entering REJOINED")
	}
	if !m.status.RecoveryInProgress {
		t.Fatalf("recovery_in_progress = false in REJOINED, want true")
	}

	if err := m.setFaultState(FaultStateHealthy, "node operating normally"); err != nil {
		t.Fatalf("setFaultState HEALTHY failed: %v", err)
	}
	if got := m.status.FaultState; got != FaultStateHealthy {
		t.Fatalf("fault state after HEALTHY = %q, want %q", got, FaultStateHealthy)
	}
	if m.rejoinedSince.IsZero() == false {
		t.Fatalf("rejoinedSince should be cleared when leaving REJOINED")
	}
	if got := m.status.LastFaultReason; got != "node operating normally" {
		t.Fatalf("last_fault_reason = %q, want %q", got, "node operating normally")
	}
	if m.status.RecoveryInProgress {
		t.Fatalf("recovery_in_progress = true in HEALTHY, want false")
	}
	if m.status.LastRecoveryTime == "" {
		t.Fatalf("last_recovery_time should be set when becoming HEALTHY after REJOINED")
	}
}

func TestSetFaultState_InvalidTransition(t *testing.T) {
	m := NewManager(Config{
		NodeID:      "A",
		ZKAddr:      "localhost:2181",
		StoragePath: "./data/nodeA/ledger.db",
	})

	// From RECOVERING, direct move to HEALTHY is intentionally invalid.
	err := m.setFaultState(FaultStateHealthy, "invalid direct jump")
	if err == nil {
		t.Fatalf("expected invalid transition error, got nil")
	}

	if got := m.status.FaultState; got != FaultStateRecovering {
		t.Fatalf("fault state changed on invalid transition: got %q, want %q", got, FaultStateRecovering)
	}
}
