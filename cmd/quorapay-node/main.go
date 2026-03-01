package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"quorapay/internal/api"
	"quorapay/internal/coordination"
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

	handler := api.NewHandler(api.Config{
		NodeID:      cfg.NodeID,
		CORSAllowed: cfg.CORSAllowed,
		ZKAddr:      cfg.ZKAddr,
		StoragePath: cfg.StoragePath,
	}, coord, store)

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
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Printf("http server shutdown failed: %v", err)
	}
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
