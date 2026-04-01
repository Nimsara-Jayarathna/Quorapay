package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"quorapay/internal/coordination"
	"quorapay/internal/payment"
	"quorapay/internal/replication"
	"quorapay/internal/storage"
	"quorapay/internal/timesync"
)

const receivedByHeader = "X-Quorapay-Received-By"

type Config struct {
	NodeID           string
	CORSAllowed      string
	ZKAddr           string
	StoragePath      string
	SkewWarnMS       int64
	SkewRejectMS     int64
	MaxMessageAgeMS  int64
	MaxFutureDriftMS int64
	LeaderHTTPClient *http.Client
	StripeSecretKey  string
}

type Coordinator interface {
	CurrentStatus() coordination.Status
	GetFollowerURLs() ([]string, error)
	AdvanceLogHead(nextIndex int64) error
	CurrentLogHead() (int64, error)
}

type Replicator interface {
	ReplicateWithQuorum(ctx context.Context, entry replication.LogEntry, followerURLs []string) (replication.QuorumReplicationResult, error)
}

type MemberDiscovery interface {
	GetMembers() ([]coordination.Member, error)
}

type LedgerStore interface {
	ListPayments(context.Context) ([]storage.Payment, error)
	ListCommittedAfter(context.Context, int64) ([]storage.Payment, error)
	AppendPending(context.Context, replication.LogEntry) error
	CommitByPaymentID(context.Context, string) error
	FailByPaymentID(context.Context, string) error
	ExistsByPaymentID(context.Context, string) (bool, error)
	GetPaymentByID(context.Context, string) (storage.Payment, error)
}

type handler struct {
	cfg              Config
	status           interface{ CurrentStatus() coordination.Status }
	coordinator      Coordinator
	ledger           LedgerStore
	replicator       Replicator
	discovery        MemberDiscovery
	leaderHTTPClient *http.Client
	lamportClock     *timesync.LamportClock
	skewTracker      *timesync.SkewTracker
	skewWarn         time.Duration
	skewReject       time.Duration
	maxMessageAge    time.Duration
	maxFutureDrift   time.Duration
	eventsMu         sync.RWMutex
	events           []PaymentEvent
	stripeClient     *payment.StripeClient
}

type PaymentEvent struct {
	Timestamp string `json:"timestamp"`
	NodeID    string `json:"node_id"`
	PaymentID string `json:"payment_id,omitempty"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
}

func NewHandler(cfg Config, status interface{ CurrentStatus() coordination.Status }, ledger LedgerStore, replicator ...Replicator) http.Handler {
	var repl Replicator
	if len(replicator) > 0 {
		repl = replicator[0]
	}

	var coord Coordinator
	if c, ok := status.(Coordinator); ok {
		coord = c
	}
	var discovery MemberDiscovery
	if d, ok := status.(MemberDiscovery); ok {
		discovery = d
	}

	leaderClient := cfg.LeaderHTTPClient
	if leaderClient == nil {
		leaderClient = &http.Client{Timeout: 5 * time.Second}
	}

	h := &handler{
		cfg:              cfg,
		status:           status,
		coordinator:      coord,
		ledger:           ledger,
		replicator:       repl,
		discovery:        discovery,
		leaderHTTPClient: leaderClient,
		lamportClock:     timesync.NewLamportClock(),
		skewTracker:      timesync.NewSkewTracker(),
		skewWarn:         durationOrDefaultMS(cfg.SkewWarnMS, 300*time.Millisecond),
		skewReject:       durationOrDefaultMS(cfg.SkewRejectMS, 500*time.Millisecond),
		maxMessageAge:    durationOrDefaultMS(cfg.MaxMessageAgeMS, 2*time.Second),
		maxFutureDrift:   durationOrDefaultMS(cfg.MaxFutureDriftMS, 500*time.Millisecond),
		stripeClient:     payment.NewStripeClient(cfg.StripeSecretKey),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/status", h.statusHandler)
	mux.HandleFunc("/ledger", h.ledgerHandler)
	mux.HandleFunc("/pay", h.payHandler)
	mux.HandleFunc("/stripe/create-checkout-session", h.stripeCreateCheckoutSessionHandler)
	mux.HandleFunc("/stripe/session-status", h.stripeSessionStatusHandler)
	mux.HandleFunc("/events", h.eventsHandler)
	mux.HandleFunc("/internal/append", h.internalAppendHandler)
	mux.HandleFunc("/internal/commit", h.internalCommitHandler)
	mux.HandleFunc("/internal/catchup", h.internalCatchUpHandler)
	mux.HandleFunc("/cluster/nodes", h.clusterNodesHandler)
	return withCORS(cfg.CORSAllowed, mux)
}

type stripeCreateCheckoutSessionRequest struct {
	PaymentID  string  `json:"payment_id"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
	SuccessURL string  `json:"success_url"`
	CancelURL  string  `json:"cancel_url"`
}

func (h *handler) stripeCreateCheckoutSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}
	if h.stripeClient == nil || !h.stripeClient.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "stripe is not configured on this node"})
		return
	}

	var req stripeCreateCheckoutSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.PaymentID) == "" || req.Amount <= 0 || strings.TrimSpace(req.Currency) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "payment_id, amount, and currency are required"})
		return
	}
	if _, err := url.ParseRequestURI(strings.TrimSpace(req.SuccessURL)); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid success_url"})
		return
	}
	if _, err := url.ParseRequestURI(strings.TrimSpace(req.CancelURL)); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid cancel_url"})
		return
	}

	session, err := h.stripeClient.CreateCheckoutSession(r.Context(), payment.CheckoutSessionCreateRequest{
		PaymentID:  strings.TrimSpace(req.PaymentID),
		Amount:     req.Amount,
		Currency:   strings.TrimSpace(req.Currency),
		SuccessURL: strings.TrimSpace(req.SuccessURL),
		CancelURL:  strings.TrimSpace(req.CancelURL),
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"message": err.Error()})
		return
	}

	h.recordEvent(PaymentEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		NodeID:    h.cfg.NodeID,
		PaymentID: req.PaymentID,
		Stage:     "STRIPE_SESSION_CREATED",
		Message:   "stripe checkout session created",
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": session.ID,
		"url":        session.URL,
	})
}

func (h *handler) stripeSessionStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}
	if h.stripeClient == nil || !h.stripeClient.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "stripe is not configured on this node"})
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "session_id is required"})
		return
	}
	session, err := h.stripeClient.GetCheckoutSession(r.Context(), sessionID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"message": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id":     session.ID,
		"payment_status": session.PaymentStatus,
		"amount_total":   session.AmountTotal,
		"currency":       strings.ToUpper(session.Currency),
		"metadata":       session.Metadata,
	})
}

func (h *handler) clusterNodesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}
	if h.discovery == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "member discovery is not available"})
		return
	}
	members, err := h.discovery.GetMembers()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(members),
		"items": members,
	})
}

func durationOrDefaultMS(valueMS int64, fallback time.Duration) time.Duration {
	if valueMS <= 0 {
		return fallback
	}
	return time.Duration(valueMS) * time.Millisecond
}

func (h *handler) payHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}

	if h.coordinator == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "coordinator is not configured"})
		return
	}

	var req replication.PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}
	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return
	}

	status := h.coordinator.CurrentStatus()
	receivedBy := strings.TrimSpace(r.Header.Get(receivedByHeader))
	if receivedBy == "" {
		receivedBy = h.cfg.NodeID
	}
	h.recordEvent(PaymentEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		NodeID:    h.cfg.NodeID,
		PaymentID: req.PaymentID,
		Stage:     "RECEIVED",
		Message:   "payment request received",
	})
	if status.Role != coordination.RoleLeader {
		if status.LeaderURL != "" {
			h.recordEvent(PaymentEvent{
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				NodeID:    h.cfg.NodeID,
				PaymentID: req.PaymentID,
				Stage:     "FORWARDED_TO_LEADER",
				Message:   "request forwarded to leader",
			})
			h.forwardPayToLeader(w, r.Context(), status.LeaderURL, receivedBy, req)
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "no leader available"})
		return
	}
	h.recordEvent(PaymentEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		NodeID:    h.cfg.NodeID,
		PaymentID: req.PaymentID,
		Stage:     "LEADER_PROCESSING",
		Message:   "leader started payment processing",
	})

	// Optional simulation toggle for demoing successful and failed transactions from UI.
	switch strings.ToUpper(strings.TrimSpace(req.SimulateOutcome)) {
	case "", "SUCCESS":
	case "FAIL", "FAILED":
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "simulated provider rejection"})
		h.recordEvent(PaymentEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			NodeID:    h.cfg.NodeID,
			PaymentID: req.PaymentID,
			Stage:     "PROVIDER_FAILED",
			Message:   "payment provider rejected the transaction",
		})
		return
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "simulate_outcome must be SUCCESS or FAILED"})
		return
	}

	payment, err := h.ledger.GetPaymentByID(r.Context(), req.PaymentID)
	if err == nil {
		if payment.Status == replication.StatusCommitted.String() {
			writeJSON(w, http.StatusOK, replication.PaymentResponse{
				Status:    "OK",
				PaymentID: req.PaymentID,
				Message:   "payment already processed",
			})
			return
		}
		if payment.Status == replication.StatusPending.String() {
			writeJSON(w, http.StatusConflict, map[string]string{
				"message": "payment is pending — quorum was not reached on previous attempt, please retry",
			})
			return
		}
		if payment.Status == replication.StatusFailed.String() {
			writeJSON(w, http.StatusConflict, map[string]string{
				"message": "payment is marked failed from previous attempt, retry with a new payment id",
			})
			return
		}
	} else if !errors.Is(err, storage.ErrPaymentNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "failed to check payment id"})
		return
	}

	currentLogHead, err := h.coordinator.CurrentLogHead()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "failed to assign log index"})
		return
	}
	nextIndex := currentLogHead + 1
	if err := h.coordinator.AdvanceLogHead(nextIndex); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "failed to assign log index"})
		return
	}

	entry := replication.LogEntry{
		LogIndex:     nextIndex,
		Term:         status.Term,
		LeaderID:     status.NodeID,
		ReceivedBy:   receivedBy,
		PaymentID:    req.PaymentID,
		Amount:       req.Amount,
		Currency:     req.Currency,
		Status:       replication.StatusPending,
		PhysicalTime: time.Now().UnixNano(),
		LogicalTime:  int64(h.lamportClock.Send()),
	}

	followerURLs, err := h.coordinator.GetFollowerURLs()
	if err != nil {
		log.Printf("failed to fetch follower URLs: %v", err)
		followerURLs = []string{}
	}

	if h.replicator == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "replicator is not configured"})
		return
	}

	result, err := h.replicator.ReplicateWithQuorum(r.Context(), entry, followerURLs)
	if err != nil {
		_ = h.ledger.FailByPaymentID(r.Context(), entry.PaymentID)
		h.recordEvent(PaymentEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			NodeID:    h.cfg.NodeID,
			PaymentID: entry.PaymentID,
			Stage:     "REPLICATION_FAILED",
			Message:   err.Error(),
		})
		if !result.QuorumReached {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "quorum not reached"})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": err.Error()})
		return
	}

	if !result.QuorumReached {
		_ = h.ledger.FailByPaymentID(r.Context(), entry.PaymentID)
		h.recordEvent(PaymentEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			NodeID:    h.cfg.NodeID,
			PaymentID: entry.PaymentID,
			Stage:     "QUORUM_NOT_REACHED",
			Message:   "payment failed because quorum was not reached",
		})
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "quorum not reached"})
		return
	}

	h.recordEvent(PaymentEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		NodeID:    h.cfg.NodeID,
		PaymentID: entry.PaymentID,
		Stage:     "COMMITTED",
		Message:   "payment committed successfully",
	})

	writeJSON(w, http.StatusOK, replication.PaymentResponse{
		Status:    "OK",
		PaymentID: entry.PaymentID,
		LogIndex:  entry.LogIndex,
		Term:      entry.Term,
		LeaderID:  entry.LeaderID,
		Trace: &replication.PaymentTrace{
			ReceivedBy:      receivedBy,
			RoutedToLeader:  receivedBy != entry.LeaderID,
			RequiredQuorum:  result.RequiredQuorum,
			AckCount:        result.AckCount,
			FollowerResults: result.FollowerResults,
		},
	})
}

func (h *handler) eventsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}
	paymentID := strings.TrimSpace(r.URL.Query().Get("payment_id"))

	h.eventsMu.RLock()
	defer h.eventsMu.RUnlock()
	items := make([]PaymentEvent, 0, len(h.events))
	for _, event := range h.events {
		if paymentID == "" || event.PaymentID == paymentID {
			items = append(items, event)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(items),
		"items": items,
	})
}

func (h *handler) recordEvent(event PaymentEvent) {
	h.eventsMu.Lock()
	defer h.eventsMu.Unlock()
	h.events = append(h.events, event)
	if len(h.events) > 250 {
		h.events = h.events[len(h.events)-250:]
	}
}

func (h *handler) forwardPayToLeader(w http.ResponseWriter, ctx context.Context, leaderURL string, receivedBy string, reqBody replication.PaymentRequest) {
	endpoint := strings.TrimRight(leaderURL, "/") + "/pay"
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "failed to reach leader"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(receivedByHeader, receivedBy)

	resp, err := h.leaderHTTPClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "failed to reach leader"})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "failed to read leader response"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)

	stage := "LEADER_RESPONSE_SUCCESS"
	msg := "leader completed forwarded payment request"
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		stage = "LEADER_RESPONSE_FAILED"
		msg = "leader returned a failed payment response"
	}
	h.recordEvent(PaymentEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		NodeID:    h.cfg.NodeID,
		PaymentID: reqBody.PaymentID,
		Stage:     stage,
		Message:   msg,
	})
}
