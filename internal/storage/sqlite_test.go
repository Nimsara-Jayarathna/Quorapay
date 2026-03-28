package storage

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"quorapay/internal/replication"
)

func TestSQLiteStore_AppendPendingSuccess(t *testing.T) {
	store := newTestStore(t)
	t.Cleanup(func() { _ = store.Close() })

	entry := replication.LogEntry{
		LogIndex:     1,
		Term:         1,
		LeaderID:     "A",
		PaymentID:    "pay-append-1",
		Amount:       10.5,
		Currency:     "USD",
		Status:       replication.StatusPending,
		PhysicalTime: 1000,
		LogicalTime:  5,
	}

	if err := store.AppendPending(context.Background(), entry); err != nil {
		t.Fatalf("AppendPending() error = %v", err)
	}

	stored, err := store.GetPaymentByID(context.Background(), entry.PaymentID)
	if err != nil {
		t.Fatalf("GetPaymentByID() error = %v", err)
	}
	if stored.Status != replication.StatusPending.String() {
		t.Fatalf("stored status = %q, want %q", stored.Status, replication.StatusPending)
	}
	if stored.LogIndex != entry.LogIndex {
		t.Fatalf("stored log_index = %d, want %d", stored.LogIndex, entry.LogIndex)
	}
	if stored.ReceivedBy != entry.LeaderID {
		t.Fatalf("stored received_by = %q, want %q", stored.ReceivedBy, entry.LeaderID)
	}
}

func TestSQLiteStore_AppendPendingDuplicateRejected(t *testing.T) {
	store := newTestStore(t)
	t.Cleanup(func() { _ = store.Close() })

	entry := replication.LogEntry{
		LogIndex:  2,
		Term:      1,
		LeaderID:  "A",
		PaymentID: "pay-dup-1",
		Amount:    20,
		Currency:  "USD",
		Status:    replication.StatusPending,
	}

	if err := store.AppendPending(context.Background(), entry); err != nil {
		t.Fatalf("first AppendPending() error = %v", err)
	}

	err := store.AppendPending(context.Background(), entry)
	if err == nil {
		t.Fatalf("second AppendPending() expected duplicate error")
	}
	if !errors.Is(err, ErrDuplicatePaymentID) {
		t.Fatalf("duplicate error = %v, want ErrDuplicatePaymentID", err)
	}
}

func TestSQLiteStore_CommitExistingPaymentSuccess(t *testing.T) {
	store := newTestStore(t)
	t.Cleanup(func() { _ = store.Close() })

	entry := replication.LogEntry{
		LogIndex:  3,
		Term:      1,
		LeaderID:  "A",
		PaymentID: "pay-commit-1",
		Amount:    33.3,
		Currency:  "USD",
		Status:    replication.StatusPending,
	}

	if err := store.AppendPending(context.Background(), entry); err != nil {
		t.Fatalf("AppendPending() error = %v", err)
	}

	if err := store.CommitByPaymentID(context.Background(), entry.PaymentID); err != nil {
		t.Fatalf("CommitByPaymentID() error = %v", err)
	}

	stored, err := store.GetPaymentByID(context.Background(), entry.PaymentID)
	if err != nil {
		t.Fatalf("GetPaymentByID() error = %v", err)
	}
	if stored.Status != replication.StatusCommitted.String() {
		t.Fatalf("stored status = %q, want %q", stored.Status, replication.StatusCommitted)
	}
}

func TestSQLiteStore_CommitMissingPaymentReturnsError(t *testing.T) {
	store := newTestStore(t)
	t.Cleanup(func() { _ = store.Close() })

	err := store.CommitByPaymentID(context.Background(), "pay-missing")
	if err == nil {
		t.Fatalf("CommitByPaymentID() expected missing payment error")
	}
	if !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("commit missing error = %v, want ErrPaymentNotFound", err)
	}
}

func TestSQLiteStore_GetPaymentByIDSuccess(t *testing.T) {
	store := newTestStore(t)
	t.Cleanup(func() { _ = store.Close() })

	entry := replication.LogEntry{
		LogIndex:     4,
		Term:         2,
		LeaderID:     "B",
		PaymentID:    "pay-get-1",
		Amount:       7.25,
		Currency:     "EUR",
		Status:       replication.StatusPending,
		PhysicalTime: 12345,
		LogicalTime:  42,
	}
	if err := store.AppendPending(context.Background(), entry); err != nil {
		t.Fatalf("AppendPending() error = %v", err)
	}

	stored, err := store.GetPaymentByID(context.Background(), entry.PaymentID)
	if err != nil {
		t.Fatalf("GetPaymentByID() error = %v", err)
	}

	if stored.PaymentID != entry.PaymentID {
		t.Fatalf("stored payment_id = %q, want %q", stored.PaymentID, entry.PaymentID)
	}
	if stored.Amount != entry.Amount {
		t.Fatalf("stored amount = %v, want %v", stored.Amount, entry.Amount)
	}
	if stored.PhysicalTime != entry.PhysicalTime || stored.LogicalTime != entry.LogicalTime {
		t.Fatalf("stored time fields = (%d,%d), want (%d,%d)", stored.PhysicalTime, stored.LogicalTime, entry.PhysicalTime, entry.LogicalTime)
	}
}

func TestSQLiteStore_ExistsByPaymentIDWorks(t *testing.T) {
	store := newTestStore(t)
	t.Cleanup(func() { _ = store.Close() })

	entry := replication.LogEntry{
		LogIndex:  5,
		Term:      2,
		LeaderID:  "C",
		PaymentID: "pay-exists-1",
		Amount:    12,
		Currency:  "USD",
		Status:    replication.StatusPending,
	}

	existsBefore, err := store.ExistsByPaymentID(context.Background(), entry.PaymentID)
	if err != nil {
		t.Fatalf("ExistsByPaymentID() before append error = %v", err)
	}
	if existsBefore {
		t.Fatalf("ExistsByPaymentID() before append = true, want false")
	}

	if err := store.AppendPending(context.Background(), entry); err != nil {
		t.Fatalf("AppendPending() error = %v", err)
	}

	existsAfter, err := store.ExistsByPaymentID(context.Background(), entry.PaymentID)
	if err != nil {
		t.Fatalf("ExistsByPaymentID() after append error = %v", err)
	}
	if !existsAfter {
		t.Fatalf("ExistsByPaymentID() after append = false, want true")
	}
}

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "ledger.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "requires cgo") {
			t.Skipf("sqlite3 unavailable in this environment: %v", err)
		}
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	return store
}
