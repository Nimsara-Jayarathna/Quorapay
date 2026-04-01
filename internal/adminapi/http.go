package adminapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"quorapay/internal/admin"
)

type Config struct {
	AdminToken      string
	CORSAllowed     string
	AdminScriptRoot string
	RunNodeScript  string
	KillNodeScript string
	ZKAddr         string
}

type handler struct {
	service *admin.Service
	token   string
}

func NewHandler(cfg Config) http.Handler {
	h := &handler{
		service: admin.NewService(nil, cfg.AdminScriptRoot, cfg.RunNodeScript, cfg.KillNodeScript, cfg.ZKAddr),
		token:   strings.TrimSpace(cfg.AdminToken),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/admin/node/", h.nodeAction)
	return withCORS(cfg.CORSAllowed, mux)
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *handler) nodeAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}
	if !h.authorize(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/admin/node/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "expected /admin/node/{id}/{start|stop|restart}"})
		return
	}

	result, err := h.service.Execute(r.Context(), parts[0], parts[1])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "OK",
		"node_id": result.NodeID,
		"action":  result.Action,
		"output":  result.Output,
	})
}

func (h *handler) authorize(r *http.Request) bool {
	if h.token == "" {
		return true
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(h.token)) == 1
}
