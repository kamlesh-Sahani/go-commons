package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is the official Go SDK client for interacting with the hosted Accurex Common API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// ClientOption configures the SDK client.
type ClientOption func(*Client)

// WithTimeout sets a custom HTTP timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithHTTPClient provides a custom *http.Client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// NewClient initializes a new Common API SDK client.
func NewClient(baseURL, apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// apiEnvelope represents the standard API response structure.
type apiEnvelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, dest interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("sdk: failed to encode request payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	fullURL := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("sdk: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sdk: http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("sdk: failed to read response body: %w", err)
	}

	var env apiEnvelope
	if err := json.Unmarshal(respBytes, &env); err != nil {
		if resp.StatusCode >= 400 {
			return fmt.Errorf("sdk: api error (status %d): %s", resp.StatusCode, string(respBytes))
		}
		return fmt.Errorf("sdk: failed to parse response envelope: %w", err)
	}

	if !env.Success || resp.StatusCode >= 400 {
		return fmt.Errorf("sdk: api error: %s", env.Message)
	}

	if dest != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, dest); err != nil {
			return fmt.Errorf("sdk: failed to decode response data: %w", err)
		}
	}

	return nil
}
