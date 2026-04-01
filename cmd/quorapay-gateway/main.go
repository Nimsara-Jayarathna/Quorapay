package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type config struct {
	Port        int
	CORSAllowed string
	Nodes       []string
}

type statusResponse struct {
	NodeID    string `json:"node_id"`
	Role      string `json:"role"`
	LeaderURL string `json:"leader_url"`
}

type gateway struct {
	nodes      []string
	httpClient *http.Client
}

func main() {
	cfg := loadConfig()
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC|log.Lmicroseconds)

	gw := &gateway{
		nodes: cfg.Nodes,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", gw.health)
	mux.HandleFunc("/payments/checkout-session", gw.checkoutSession)
	mux.HandleFunc("/payments/finalize", gw.finalizeSession)
	mux.HandleFunc("/payments/cancel", gw.cancelSession)
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           withCORS(cfg.CORSAllowed, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("starting quorapay gateway port=%d nodes=%s", cfg.Port, strings.Join(cfg.Nodes, ","))
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
		logger.Printf("gateway server failed: %v", err)
	}

	_ = server.Close()
}

func (g *gateway) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "timestamp": time.Now().UTC().Format(time.RFC3339)})
}

func (g *gateway) checkoutSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}
	status, payload, err := g.forwardToLeader(r.Context(), "/stripe/create-checkout-session", body)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": err.Error()})
		return
	}
	writeRawJSON(w, status, payload)
}

func (g *gateway) finalizeSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}
	status, payload, err := g.forwardToLeader(r.Context(), "/stripe/finalize-checkout-session", body)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": err.Error()})
		return
	}
	writeRawJSON(w, status, payload)
}

func (g *gateway) cancelSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}
	status, payload, err := g.forwardToLeader(r.Context(), "/stripe/cancel-checkout-session", body)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": err.Error()})
		return
	}
	writeRawJSON(w, status, payload)
}

func (g *gateway) forwardToLeader(ctx context.Context, path string, body []byte) (int, []byte, error) {
	leaderURL, err := g.findLeaderURL(ctx)
	if err != nil {
		return 0, nil, err
	}
	status, payload, err := g.forward(ctx, strings.TrimRight(leaderURL, "/")+path, body)
	if err == nil {
		return status, payload, nil
	}

	leaderURL2, err2 := g.findLeaderURL(ctx)
	if err2 != nil {
		return 0, nil, err
	}
	if leaderURL2 == leaderURL {
		return 0, nil, err
	}
	return g.forward(ctx, strings.TrimRight(leaderURL2, "/")+path, body)
}

func (g *gateway) forward(ctx context.Context, endpoint string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, payload, nil
}

func (g *gateway) findLeaderURL(ctx context.Context) (string, error) {
	for _, base := range g.nodes {
		status, err := g.fetchStatus(ctx, base)
		if err != nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(status.Role), "LEADER") {
			return strings.TrimRight(base, "/"), nil
		}
		if strings.TrimSpace(status.LeaderURL) != "" {
			return strings.TrimRight(status.LeaderURL, "/"), nil
		}
	}
	return "", errors.New("no leader available")
}

func (g *gateway) fetchStatus(ctx context.Context, base string) (statusResponse, error) {
	var out statusResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/status", nil)
	if err != nil {
		return out, err
	}
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, errors.New("status request failed")
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

func loadConfig() config {
	return config{
		Port:        getEnvInt("GATEWAY_PORT", 18100),
		CORSAllowed: getEnv("GATEWAY_CORS_ALLOWED_ORIGINS", "http://localhost:5173"),
		Nodes:       resolveNodeURLs(),
	}
}

func resolveNodeURLs() []string {
	host := getEnv("GATEWAY_NODE_HOST", "localhost")
	nodes := strings.TrimSpace(getEnv("NODES", ""))
	if nodes != "" {
		parts := strings.Split(nodes, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			p := strings.TrimSpace(part)
			if p == "" {
				continue
			}
			chunks := strings.Split(p, ":")
			if len(chunks) != 2 {
				continue
			}
			out = append(out, "http://"+host+":"+strings.TrimSpace(chunks[1]))
		}
		if len(out) > 0 {
			return out
		}
	}

	size := getEnvInt("CLUSTER_SIZE", 3)
	basePort := getEnvInt("CLUSTER_BASE_PORT", 8001)
	if size < 1 {
		size = 3
	}
	out := make([]string, 0, size)
	for i := 0; i < size; i++ {
		out = append(out, "http://"+host+":"+strconv.Itoa(basePort+i))
	}
	return out
}

func withCORS(allowedOrigin string, next http.Handler) http.Handler {
	allow := strings.TrimSpace(allowedOrigin)
	if allow == "" {
		allow = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allow)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeRawJSON(w http.ResponseWriter, status int, payload []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
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
