package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const stripeAPIBase = "https://api.stripe.com/v1"

type StripeClient struct {
	secretKey  string
	httpClient *http.Client
}

type CheckoutSessionCreateRequest struct {
	PaymentID  string
	Amount     float64
	Currency   string
	SuccessURL string
	CancelURL  string
}

type CheckoutSession struct {
	ID            string            `json:"id"`
	URL           string            `json:"url"`
	PaymentStatus string            `json:"payment_status"`
	AmountTotal   int64             `json:"amount_total"`
	Currency      string            `json:"currency"`
	Metadata      map[string]string `json:"metadata"`
}

func NewStripeClient(secretKey string) *StripeClient {
	return &StripeClient{
		secretKey: strings.TrimSpace(secretKey),
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (c *StripeClient) Enabled() bool {
	return strings.TrimSpace(c.secretKey) != ""
}

func (c *StripeClient) CreateCheckoutSession(ctx context.Context, req CheckoutSessionCreateRequest) (CheckoutSession, error) {
	if !c.Enabled() {
		return CheckoutSession{}, fmt.Errorf("stripe is not configured")
	}
	if strings.TrimSpace(req.PaymentID) == "" {
		return CheckoutSession{}, fmt.Errorf("payment_id is required")
	}
	if req.Amount <= 0 {
		return CheckoutSession{}, fmt.Errorf("amount must be positive")
	}
	currency := strings.ToLower(strings.TrimSpace(req.Currency))
	if currency == "" {
		return CheckoutSession{}, fmt.Errorf("currency is required")
	}
	if strings.TrimSpace(req.SuccessURL) == "" || strings.TrimSpace(req.CancelURL) == "" {
		return CheckoutSession{}, fmt.Errorf("success_url and cancel_url are required")
	}

	unitAmount := int64(req.Amount * 100)
	if unitAmount <= 0 {
		return CheckoutSession{}, fmt.Errorf("invalid amount")
	}

	values := url.Values{}
	values.Set("mode", "payment")
	values.Set("success_url", req.SuccessURL)
	values.Set("cancel_url", req.CancelURL)
	values.Set("line_items[0][quantity]", "1")
	values.Set("line_items[0][price_data][currency]", currency)
	values.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(unitAmount, 10))
	values.Set("line_items[0][price_data][product_data][name]", "Quorapay Order Payment")
	values.Set("metadata[payment_id]", req.PaymentID)
	values.Set("metadata[amount]", fmt.Sprintf("%.2f", req.Amount))
	values.Set("metadata[currency]", strings.ToUpper(currency))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, stripeAPIBase+"/checkout/sessions", strings.NewReader(values.Encode()))
	if err != nil {
		return CheckoutSession{}, err
	}
	httpReq.SetBasicAuth(c.secretKey, "")
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return CheckoutSession{}, fmt.Errorf("stripe request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return CheckoutSession{}, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return CheckoutSession{}, fmt.Errorf("stripe create checkout session failed: status=%d body=%s", httpResp.StatusCode, string(body))
	}

	var out CheckoutSession
	if err := json.Unmarshal(body, &out); err != nil {
		return CheckoutSession{}, err
	}
	if strings.TrimSpace(out.ID) == "" || strings.TrimSpace(out.URL) == "" {
		return CheckoutSession{}, fmt.Errorf("stripe returned incomplete checkout session")
	}
	return out, nil
}

func (c *StripeClient) GetCheckoutSession(ctx context.Context, sessionID string) (CheckoutSession, error) {
	if !c.Enabled() {
		return CheckoutSession{}, fmt.Errorf("stripe is not configured")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return CheckoutSession{}, fmt.Errorf("session_id is required")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, stripeAPIBase+"/checkout/sessions/"+url.PathEscape(sessionID), nil)
	if err != nil {
		return CheckoutSession{}, err
	}
	httpReq.SetBasicAuth(c.secretKey, "")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return CheckoutSession{}, fmt.Errorf("stripe request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return CheckoutSession{}, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return CheckoutSession{}, fmt.Errorf("stripe get checkout session failed: status=%d body=%s", httpResp.StatusCode, string(body))
	}

	var out CheckoutSession
	if err := json.Unmarshal(body, &out); err != nil {
		return CheckoutSession{}, err
	}
	return out, nil
}
