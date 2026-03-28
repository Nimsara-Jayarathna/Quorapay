package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"quorapay/internal/api"
	"quorapay/internal/coordination"
	"quorapay/internal/replication"
	"quorapay/internal/storage"
)

type config struct {
	NodeID      string
	Port        int
	BaseURL     string
	CORSAllowed string
	ZKAddr      string
	ZKRoot      string
	StoragePath string
}

func main() {
	cfg := loadConfig()
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC|log.Lmicroseconds)
	shutdownCh := make(chan string, 1)

	store, err := storage.NewSQLiteStore(cfg.StoragePath)
	if err != nil {
		logger.Fatalf("storage initialization failed: %v", err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			logger.Printf("storage close failed: %v", closeErr)
		}
	}()

	coord := coordination.NewManager(coordination.Config{
		NodeID:      cfg.NodeID,
		BaseURL:     cfg.BaseURL,
		ZKAddr:      cfg.ZKAddr,
		ZKRoot:      cfg.ZKRoot,
		StoragePath: cfg.StoragePath,
		Logger:      logger,
	})
	if err := coord.Start(); err != nil {
		logger.Printf("coordination startup warning: %v", err)
	}
	defer func() {
		if closeErr := coord.Close(); closeErr != nil {
			logger.Printf("coordination close failed: %v", closeErr)
		}
	}()

	replClient := replication.NewHTTPClient(nil)
	replService := replication.NewReplicationService(store, replClient)
	recoveryStopCh := make(chan struct{})
	defer close(recoveryStopCh)
	go runFollowerCatchUpLoop(logger, coord, store, replClient, recoveryStopCh)

	handler := api.NewHandler(api.Config{
		NodeID:      cfg.NodeID,
		CORSAllowed: cfg.CORSAllowed,
		ZKAddr:      cfg.ZKAddr,
		StoragePath: cfg.StoragePath,
		RequestShutdown: func(reason string) {
			select {
			case shutdownCh <- reason:
			default:
			}
		},
	}, coord, store, replService)

	server := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("starting quorapay node node_id=%s port=%d base_url=%s zk_addr=%s storage_path=%s", cfg.NodeID, cfg.Port, cfg.BaseURL, cfg.ZKAddr, cfg.StoragePath)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Printf("shutdown signal received: %s", sig.String())
	case serveErr := <-errCh:
		logger.Printf("http server failed: %v", serveErr)
	case reason := <-shutdownCh:
		logger.Printf("shutdown requested via admin endpoint: %s", reason)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Printf("http server shutdown failed: %v", err)
	}
}

func runFollowerCatchUpLoop(
	logger *log.Logger,
	coord interface{ CurrentStatus() coordination.Status },
	store interface {
		ListCommittedAfter(context.Context, int64) ([]storage.Payment, error)
		AppendPending(context.Context, replication.LogEntry) error
		CommitByPaymentID(context.Context, string) error
	},
	client *replication.HTTPClient,
	stopCh <-chan struct{},
) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
		}

		status := coord.CurrentStatus()
		if status.Role == coordination.RoleLeader || strings.TrimSpace(status.LeaderURL) == "" {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		lastCommittedIndex, err := readLocalCommittedHead(ctx, store)
		if err != nil {
			cancel()
			logger.Printf("catch-up skipped: failed to read local head: %v", err)
			continue
		}

		resp, err := client.CatchUpFromLeader(ctx, status.LeaderURL, lastCommittedIndex)
		cancel()
		if err != nil {
			logger.Printf("catch-up request failed from leader=%s: %v", status.LeaderURL, err)
			continue
		}
		if !resp.Success || len(resp.Entries) == 0 {
			continue
		}

		applied := 0
		for _, entry := range resp.Entries {
			entry.Status = replication.StatusPending
			if strings.TrimSpace(entry.ReceivedBy) == "" {
				entry.ReceivedBy = entry.LeaderID
			}
			if err := store.AppendPending(context.Background(), entry); err != nil && !errors.Is(err, storage.ErrDuplicatePaymentID) {
				logger.Printf("catch-up append failed payment_id=%s: %v", entry.PaymentID, err)
				continue
			}
			if err := store.CommitByPaymentID(context.Background(), entry.PaymentID); err != nil && !errors.Is(err, storage.ErrPaymentNotFound) {
				logger.Printf("catch-up commit failed payment_id=%s: %v", entry.PaymentID, err)
				continue
			}
			applied++
		}
		if applied > 0 {
			logger.Printf("catch-up applied %d entries from leader=%s", applied, status.LeaderURL)
		}
	}
}

func readLocalCommittedHead(
	ctx context.Context,
	store interface {
		ListCommittedAfter(context.Context, int64) ([]storage.Payment, error)
	},
) (int64, error) {
	items, err := store.ListCommittedAfter(ctx, 0)
	if err != nil {
		return 0, err
	}
	var max int64
	for _, item := range items {
		if item.LogIndex > max {
			max = item.LogIndex
		}
	}
	return max, nil
}

func loadConfig() config {
	port := getEnvInt("PORT", 8001)
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + strconv.Itoa(port)
	}

	return config{
		NodeID:      getEnv("NODE_ID", "A"),
		Port:        port,
		BaseURL:     baseURL,
		CORSAllowed: getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173"),
		ZKAddr:      getEnv("ZK_ADDR", "localhost:2181"),
		ZKRoot:      getEnv("ZK_ROOT", "/quorapay"),
		StoragePath: getEnv("STORAGE_PATH", "./data/nodeA/ledger.db"),
	}
}

func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
