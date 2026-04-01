package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"quorapay/internal/adminapi"
)

type config struct {
	Port            int
	AdminToken      string
	CORSAllowed     string
	AdminScriptRoot string
	RunNodeScript   string
	KillNodeScript  string
	ZKAddr          string
}

func main() {
	cfg := loadConfig()
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC|log.Lmicroseconds)

	handler := adminapi.NewHandler(adminapi.Config{
		AdminToken:      cfg.AdminToken,
		CORSAllowed:     cfg.CORSAllowed,
		AdminScriptRoot: cfg.AdminScriptRoot,
		RunNodeScript:   cfg.RunNodeScript,
		KillNodeScript:  cfg.KillNodeScript,
		ZKAddr:          cfg.ZKAddr,
	})

	server := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("starting quorapay admin service port=%d zk_addr=%s script_root=%s", cfg.Port, cfg.ZKAddr, cfg.AdminScriptRoot)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Printf("shutdown signal received: %s", sig.String())
	case err := <-errCh:
		logger.Printf("admin server failed: %v", err)
	}

	_ = server.Close()
}

func loadConfig() config {
	return config{
		Port:            getEnvInt("ADMIN_PORT", 8090),
		AdminToken:      getEnv("ADMIN_API_TOKEN", ""),
		CORSAllowed:     getEnv("ADMIN_CORS_ALLOWED_ORIGINS", "http://localhost:5173"),
		AdminScriptRoot: getEnv("ADMIN_SCRIPT_ROOT", "."),
		RunNodeScript:   getEnv("RUN_NODE_SCRIPT", "./scripts/run-node.sh"),
		KillNodeScript:  getEnv("KILL_NODE_SCRIPT", "./scripts/kill-node.sh"),
		ZKAddr:          getEnv("ZK_ADDR", "localhost:2181"),
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
