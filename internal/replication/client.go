package replication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	appendPath     = "/internal/append"
	commitPath     = "/internal/commit"
	defaultTimeout = 5 * time.Second
)

// HTTPClient sends replication commands from a leader to follower nodes.
type HTTPClient struct {
	httpClient *http.Client
}

// NewHTTPClient creates a reusable replication transport client.
// If httpClient is nil, a default client with a conservative timeout is used.
func NewHTTPClient(httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &HTTPClient{httpClient: httpClient}
}

func (c *HTTPClient) AppendToFollower(ctx context.Context, followerBaseURL string, req AppendEntriesRequest) (AppendEntriesResponse, error) {
	var resp AppendEntriesResponse
	endpoint, err := joinFollowerURL(followerBaseURL, appendPath)
	if err != nil {
		return resp, fmt.Errorf("build append URL: %w", err)
	}

	if err := c.postJSON(ctx, endpoint, req, &resp); err != nil {
		return AppendEntriesResponse{}, fmt.Errorf("append to follower %s: %w", followerBaseURL, err)
	}

	return resp, nil
}

func (c *HTTPClient) CommitToFollower(ctx context.Context, followerBaseURL string, req CommitRequest) (CommitResponse, error) {
	var resp CommitResponse
	endpoint, err := joinFollowerURL(followerBaseURL, commitPath)
	if err != nil {
		return resp, fmt.Errorf("build commit URL: %w", err)
	}

	if err := c.postJSON(ctx, endpoint, req, &resp); err != nil {
		return CommitResponse{}, fmt.Errorf("commit to follower %s: %w", followerBaseURL, err)
	}

	return resp, nil
}

func (c *HTTPClient) postJSON(ctx context.Context, endpoint string, request any, out any) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("construct request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("follower returned status %d: %s", httpResp.StatusCode, extractResponseMessage(bodyBytes))
	}

	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return fmt.Errorf("decode response JSON: %w", err)
	}

	return nil
}

func joinFollowerURL(base string, requestPath string) (string, error) {
	if strings.TrimSpace(base) == "" {
		return "", fmt.Errorf("base URL is required")
	}

	parsedBase, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if parsedBase.Scheme == "" || parsedBase.Host == "" {
		return "", fmt.Errorf("base URL must include scheme and host")
	}

	parsedPath, err := url.Parse(requestPath)
	if err != nil {
		return "", err
	}

	return parsedBase.ResolveReference(parsedPath).String(), nil
}

func extractResponseMessage(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "empty response body"
	}

	var payload struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if payload.Message != "" {
			return payload.Message
		}
		if payload.Error != "" {
			return payload.Error
		}
	}

	return trimmed
}
