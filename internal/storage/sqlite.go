package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Payment struct {
	ID          int64   `json:"id"`
	PaymentID   string  `json:"payment_id"`
	LogIndex    int64   `json:"log_index"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	ProcessedBy string  `json:"processed_by"`
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
		SELECT id, payment_id, log_index, amount, currency, status, created_at, processed_by
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
			&payment.CreatedAt,
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

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS payments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			payment_id TEXT UNIQUE,
			log_index INTEGER,
			amount REAL,
			currency TEXT,
			status TEXT,
			created_at TEXT,
			processed_by TEXT
		)
	`)
	return err
}
