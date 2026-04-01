package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
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
	CancelByPaymentID(context.Context, string) error
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
	paymentLockMu    sync.Mutex
	paymentLocks     map[string]*sync.Mutex
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
		paymentLocks:     make(map[string]*sync.Mutex),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/status", h.statusHandler)
	mux.HandleFunc("/ledger", h.ledgerHandler)
	mux.HandleFunc("/pay", h.payHandler)
	mux.HandleFunc("/stripe/create-checkout-session", h.stripeCreateCheckoutSessionHandler)
	mux.HandleFunc("/stripe/session-status", h.stripeSessionStatusHandler)
	mux.HandleFunc("/stripe/finalize-checkout-session", h.stripeFinalizeCheckoutSessionHandler)
	mux.HandleFunc("/stripe/cancel-checkout-session", h.stripeCancelCheckoutSessionHandler)
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

type stripeFinalizeCheckoutSessionRequest struct {
	SessionID string `json:"session_id"`
}

type stripeCancelCheckoutSessionRequest struct {
	PaymentID string  `json:"payment_id"`
	Reason    string  `json:"reason,omitempty"`
	Amount    float64 `json:"amount,omitempty"`
	Currency  string  `json:"currency,omitempty"`
}

func (h *handler) stripeFinalizeCheckoutSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}
	if h.stripeClient == nil || !h.stripeClient.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "stripe is not configured on this node"})
		return
	}

	var req stripeFinalizeCheckoutSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.SessionID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "session_id is required"})
		return
	}

	session, err := h.stripeClient.GetCheckoutSession(r.Context(), req.SessionID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"message": err.Error()})
		return
	}
	if strings.ToLower(strings.TrimSpace(session.PaymentStatus)) != "paid" {
		writeJSON(w, http.StatusConflict, map[string]string{"message": "stripe session is not paid"})
		return
	}

	paymentID := strings.TrimSpace(session.Metadata["payment_id"])
	amountRaw := strings.TrimSpace(session.Metadata["amount"])
	currency := strings.ToUpper(strings.TrimSpace(session.Metadata["currency"]))
	amount, err := strconv.ParseFloat(amountRaw, 64)
	if paymentID == "" || amount <= 0 || currency == "" || err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "stripe metadata is incomplete"})
		return
	}

	resp, code, err := h.processPaymentRequest(r.Context(), replication.PaymentRequest{
		PaymentID: paymentID,
		Amount:    amount,
		Currency:  currency,
	}, h.cfg.NodeID)
	if err != nil {
		writeJSON(w, code, map[string]string{"message": err.Error()})
		return
	}
	writeJSON(w, code, resp)
}

func (h *handler) stripeCancelCheckoutSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "method not allowed"})
		return
	}
	var req stripeCancelCheckoutSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.PaymentID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "payment_id is required"})
		return
	}
	status := h.coordinator.CurrentStatus()
	if status.Role != coordination.RoleLeader {
		if status.LeaderURL == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": "no leader available"})
			return
		}
		code, payload, err := h.forwardJSON(r.Context(), strings.TrimRight(status.LeaderURL, "/")+"/stripe/cancel-checkout-session", req)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"message": err.Error()})
			return
		}
		writeRawJSON(w, code, payload)
		return
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "checkout canceled by user"
	}

	if _, err := h.ledger.GetPaymentByID(r.Context(), req.PaymentID); err != nil {
		if !errors.Is(err, storage.ErrPaymentNotFound) {
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
		currency := strings.ToUpper(strings.TrimSpace(req.Currency))
		if currency == "" {
			currency = "USD"
		}
		if req.Amount < 0 {
			req.Amount = 0
		}
		if err := h.ledger.AppendPending(r.Context(), replication.LogEntry{
			LogIndex:     nextIndex,
			Term:         status.Term,
			LeaderID:     status.NodeID,
			ReceivedBy:   status.NodeID,
			PaymentID:    req.PaymentID,
			Amount:       req.Amount,
			Currency:     currency,
			Status:       replication.StatusPending,
			PhysicalTime: time.Now().UnixNano(),
			LogicalTime:  int64(h.lamportClock.Send()),
		}); err != nil && !errors.Is(err, storage.ErrDuplicatePaymentID) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": err.Error()})
			return
		}
	}
	_ = h.ledger.CancelByPaymentID(r.Context(), req.PaymentID)

	h.recordEvent(PaymentEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		NodeID:    h.cfg.NodeID,
		PaymentID: req.PaymentID,
		Stage:     "STRIPE_CHECKOUT_CANCELED",
		Message:   reason,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "OK",
		"payment_id": req.PaymentID,
		"message":    "cancellation recorded",
	})
}

func writeRawJSON(w http.ResponseWriter, status int, payload []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func (h *handler) forwardJSON(ctx context.Context, endpoint string, body any) (int, []byte, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.leaderHTTPClient.Do(req)
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

	receivedBy := strings.TrimSpace(r.Header.Get(receivedByHeader))
	if receivedBy == "" {
		receivedBy = h.cfg.NodeID
	}
	resp, code, err := h.processPaymentRequest(r.Context(), req, receivedBy)
	if err != nil {
		writeJSON(w, code, map[string]string{"message": err.Error()})
		return
	}
	writeJSON(w, code, resp)
}

func (h *handler) processPaymentRequest(ctx context.Context, req replication.PaymentRequest, receivedBy string) (replication.PaymentResponse, int, error) {
	unlock := h.lockPaymentID(req.PaymentID)
	defer unlock()

	status := h.coordinator.CurrentStatus()
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
			code, body, fwdErr := h.forwardPayToLeaderJSON(ctx, status.LeaderURL, receivedBy, req)
			if fwdErr != nil {
				// leader may have changed; resolve again and retry once
				latest := h.coordinator.CurrentStatus()
				if strings.TrimSpace(latest.LeaderURL) != "" && latest.LeaderURL != status.LeaderURL {
					code, body, fwdErr = h.forwardPayToLeaderJSON(ctx, latest.LeaderURL, receivedBy, req)
				}
			}
			if fwdErr != nil {
				return replication.PaymentResponse{}, http.StatusServiceUnavailable, fmt.Errorf("failed to reach leader")
			}
			if code < 200 || code >= 300 {
				var out map[string]string
				_ = json.Unmarshal(body, &out)
				msg := strings.TrimSpace(out["message"])
				if msg == "" {
					msg = "leader returned error"
				}
				return replication.PaymentResponse{}, code, fmt.Errorf(msg)
			}
			var out replication.PaymentResponse
			if err := json.Unmarshal(body, &out); err != nil {
				return replication.PaymentResponse{}, http.StatusServiceUnavailable, fmt.Errorf("invalid leader response")
			}
			return out, http.StatusOK, nil
		}
		return replication.PaymentResponse{}, http.StatusServiceUnavailable, fmt.Errorf("no leader available")
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
		h.recordEvent(PaymentEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			NodeID:    h.cfg.NodeID,
			PaymentID: req.PaymentID,
			Stage:     "PROVIDER_FAILED",
			Message:   "payment provider rejected the transaction",
		})
		return replication.PaymentResponse{}, http.StatusBadRequest, fmt.Errorf("simulated provider rejection")
	default:
		return replication.PaymentResponse{}, http.StatusBadRequest, fmt.Errorf("simulate_outcome must be SUCCESS or FAILED")
	}

	payment, err := h.ledger.GetPaymentByID(ctx, req.PaymentID)
	if err == nil {
		if payment.Status == replication.StatusCommitted.String() {
			return replication.PaymentResponse{
				Status:    "OK",
				PaymentID: req.PaymentID,
				Message:   "payment already processed",
			}, http.StatusOK, nil
		}
		if payment.Status == replication.StatusPending.String() {
			return replication.PaymentResponse{}, http.StatusConflict, fmt.Errorf("payment is pending — quorum was not reached on previous attempt, please retry")
		}
		if payment.Status == replication.StatusFailed.String() {
			return replication.PaymentResponse{}, http.StatusConflict, fmt.Errorf("payment is marked failed from previous attempt, retry with a new payment id")
		}
	} else if !errors.Is(err, storage.ErrPaymentNotFound) {
		return replication.PaymentResponse{}, http.StatusInternalServerError, fmt.Errorf("failed to check payment id")
	}

	currentLogHead, err := h.coordinator.CurrentLogHead()
	if err != nil {
		return replication.PaymentResponse{}, http.StatusServiceUnavailable, fmt.Errorf("failed to assign log index")
	}
	nextIndex := currentLogHead + 1
	if err := h.coordinator.AdvanceLogHead(nextIndex); err != nil {
		return replication.PaymentResponse{}, http.StatusServiceUnavailable, fmt.Errorf("failed to assign log index")
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
		return replication.PaymentResponse{}, http.StatusServiceUnavailable, fmt.Errorf("replicator is not configured")
	}

	result, err := h.replicator.ReplicateWithQuorum(ctx, entry, followerURLs)
	if err != nil {
		_ = h.ledger.FailByPaymentID(ctx, entry.PaymentID)
		h.recordEvent(PaymentEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			NodeID:    h.cfg.NodeID,
			PaymentID: entry.PaymentID,
			Stage:     "REPLICATION_FAILED",
			Message:   err.Error(),
		})
		if !result.QuorumReached {
			return replication.PaymentResponse{}, http.StatusServiceUnavailable, fmt.Errorf("quorum not reached")
		}
		return replication.PaymentResponse{}, http.StatusServiceUnavailable, err
	}

	if !result.QuorumReached {
		_ = h.ledger.FailByPaymentID(ctx, entry.PaymentID)
		h.recordEvent(PaymentEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			NodeID:    h.cfg.NodeID,
			PaymentID: entry.PaymentID,
			Stage:     "QUORUM_NOT_REACHED",
			Message:   "payment failed because quorum was not reached",
		})
		return replication.PaymentResponse{}, http.StatusServiceUnavailable, fmt.Errorf("quorum not reached")
	}

	h.recordEvent(PaymentEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		NodeID:    h.cfg.NodeID,
		PaymentID: entry.PaymentID,
		Stage:     "COMMITTED",
		Message:   "payment committed successfully",
	})

	return replication.PaymentResponse{
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
	}, http.StatusOK, nil
}

func (h *handler) lockPaymentID(paymentID string) func() {
	key := strings.TrimSpace(paymentID)
	if key == "" {
		return func() {}
	}

	h.paymentLockMu.Lock()
	mu, ok := h.paymentLocks[key]
	if !ok {
		mu = &sync.Mutex{}
		h.paymentLocks[key] = mu
	}
	h.paymentLockMu.Unlock()

	mu.Lock()
	return mu.Unlock
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

func (h *handler) forwardPayToLeaderJSON(ctx context.Context, leaderURL string, receivedBy string, reqBody replication.PaymentRequest) (int, []byte, error) {
	endpoint := strings.TrimRight(leaderURL, "/") + "/pay"
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(receivedByHeader, receivedBy)
	resp, err := h.leaderHTTPClient.Do(req)
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
