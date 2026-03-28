package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"quorapay/internal/replication"
)

var (
	ErrDuplicatePaymentID = errors.New("payment already exists")
	ErrPaymentNotFound    = errors.New("payment not found")
)

type Payment struct {
	ID           int64   `json:"id"`
	PaymentID    string  `json:"payment_id"`
	LogIndex     int64   `json:"log_index"`
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	Status       string  `json:"status"`
	PhysicalTime int64   `json:"physical_time,omitempty"`
	LogicalTime  int64   `json:"logical_time,omitempty"`
	CreatedAt    string  `json:"created_at"`
	ReceivedBy   string  `json:"received_by"`
	ProcessedBy  string  `json:"processed_by"`
}

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) ListPayments(ctx context.Context) ([]Payment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, payment_id, log_index, amount, currency, status, physical_time, logical_time, created_at, received_by, processed_by
		FROM payments
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Payment, 0)
	for rows.Next() {
		var payment Payment
		if err := rows.Scan(
			&payment.ID,
			&payment.PaymentID,
			&payment.LogIndex,
			&payment.Amount,
			&payment.Currency,
			&payment.Status,
			&payment.PhysicalTime,
			&payment.LogicalTime,
			&payment.CreatedAt,
			&payment.ReceivedBy,
			&payment.ProcessedBy,
		); err != nil {
			return nil, err
		}
		items = append(items, payment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *SQLiteStore) AppendPending(ctx context.Context, entry replication.LogEntry) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("invalid log entry: %w", err)
	}

	if entry.Status == "" {
		entry.Status = replication.StatusPending
	}
	if entry.Status != replication.StatusPending {
		return fmt.Errorf("append requires %s status", replication.StatusPending)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	receivedBy := entry.ReceivedBy
	if strings.TrimSpace(receivedBy) == "" {
		receivedBy = entry.LeaderID
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO payments (payment_id, log_index, amount, currency, status, physical_time, logical_time, created_at, received_by, processed_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.PaymentID,
		entry.LogIndex,
		entry.Amount,
		entry.Currency,
		entry.Status.String(),
		entry.PhysicalTime,
		entry.LogicalTime,
		now,
		receivedBy,
		entry.LeaderID,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return fmt.Errorf("%w: payment_id=%s", ErrDuplicatePaymentID, entry.PaymentID)
		}
		return fmt.Errorf("append pending payment: %w", err)
	}

	return nil
}

func (s *SQLiteStore) CommitByPaymentID(ctx context.Context, paymentID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE payments
		SET status = ?
		WHERE payment_id = ? AND status = ?
	`, replication.StatusCommitted.String(), paymentID, replication.StatusPending.String())
	if err != nil {
		return fmt.Errorf("commit payment %s: %w", paymentID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read commit result for payment %s: %w", paymentID, err)
	}
	if rowsAffected > 0 {
		return nil
	}

	payment, err := s.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return err
	}
	if payment.Status == replication.StatusCommitted.String() {
		return nil
	}

	return fmt.Errorf("commit payment %s: current status is %s", paymentID, payment.Status)
}

func (s *SQLiteStore) GetPaymentByID(ctx context.Context, paymentID string) (Payment, error) {
	var payment Payment
	err := s.db.QueryRowContext(ctx, `
		SELECT id, payment_id, log_index, amount, currency, status, physical_time, logical_time, created_at, received_by, processed_by
		FROM payments
		WHERE payment_id = ?
	`, paymentID).Scan(
		&payment.ID,
		&payment.PaymentID,
		&payment.LogIndex,
		&payment.Amount,
		&payment.Currency,
		&payment.Status,
		&payment.PhysicalTime,
		&payment.LogicalTime,
		&payment.CreatedAt,
		&payment.ReceivedBy,
		&payment.ProcessedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Payment{}, fmt.Errorf("%w: payment_id=%s", ErrPaymentNotFound, paymentID)
		}
		return Payment{}, fmt.Errorf("get payment %s: %w", paymentID, err)
	}

	return payment, nil
}

func (s *SQLiteStore) ExistsByPaymentID(ctx context.Context, paymentID string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM payments WHERE payment_id = ?)
	`, paymentID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check payment existence %s: %w", paymentID, err)
	}

	return exists == 1, nil
}

func (s *SQLiteStore) ListCommittedAfter(ctx context.Context, logIndex int64) ([]Payment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, payment_id, log_index, amount, currency, status, physical_time, logical_time, created_at, received_by, processed_by
		FROM payments
		WHERE status = ? AND log_index > ?
		ORDER BY log_index ASC, id ASC
	`, replication.StatusCommitted.String(), logIndex)
	if err != nil {
		return nil, fmt.Errorf("list committed payments after log index %d: %w", logIndex, err)
	}
	defer rows.Close()

	items := make([]Payment, 0)
	for rows.Next() {
		var payment Payment
		if err := rows.Scan(
			&payment.ID,
			&payment.PaymentID,
			&payment.LogIndex,
			&payment.Amount,
			&payment.Currency,
			&payment.Status,
			&payment.PhysicalTime,
			&payment.LogicalTime,
			&payment.CreatedAt,
			&payment.ReceivedBy,
			&payment.ProcessedBy,
		); err != nil {
			return nil, fmt.Errorf("scan committed payment row: %w", err)
		}
		items = append(items, payment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate committed payments: %w", err)
	}

	return items, nil
}

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS payments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			payment_id TEXT UNIQUE,
			log_index INTEGER,
			amount REAL,
			currency TEXT,
			status TEXT,
			physical_time INTEGER DEFAULT 0,
			logical_time INTEGER DEFAULT 0,
			created_at TEXT,
			received_by TEXT,
			processed_by TEXT
		)
	`)
	if err != nil {
		return err
	}

	if err := s.ensureColumnExists("payments", "physical_time", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumnExists("payments", "logical_time", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumnExists("payments", "received_by", "TEXT DEFAULT ''"); err != nil {
		return err
	}

	if _, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_payment_id ON payments(payment_id)`); err != nil {
		return err
	}

	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_log_index ON payments(log_index)`); err != nil {
		return err
	}

	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_status_log_index ON payments(status, log_index)`); err != nil {
		return err
	}

	return nil
}

func (s *SQLiteStore) ensureColumnExists(tableName string, columnName string, columnDef string) error {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, columnName) {
			return nil
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	_, err = s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnDef))
	if err != nil {
		return err
	}

	return nil
}

func isUniqueConstraintError(err error) bool {
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "unique") || strings.Contains(errText, "constraint")
}
