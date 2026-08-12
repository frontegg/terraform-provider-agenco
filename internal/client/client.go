package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	appIntegrationsPrefix = "/app-integrations"
	applicationsPrefix    = "/applications"
	identityPrefix        = "/identity"
	requestTimeout        = 60 * time.Second
	tokenRefreshBuffer    = 60 * time.Second
)

// Client is an authenticated Frontegg vendor-scoped API client.
type Client struct {
	baseURL    string
	clientID   string
	secret     string
	httpClient *http.Client

	mu          sync.RWMutex
	accessToken string
	tokenExpiry time.Time
}

// APIError carries the status code so callers can distinguish "not found" from real failures.
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Body       string
	TraceID    string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("%s %s returned %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
	if e.TraceID != "" {
		msg += fmt.Sprintf(" (frontegg-trace-id: %s)", e.TraceID)
	}
	return msg
}

// IsNotFound reports whether err is an APIError with a 404 status.
func IsNotFound(err error) bool {
	apiErr, ok := err.(*APIError)
	return ok && apiErr.StatusCode == http.StatusNotFound
}

type authResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expiresIn"`
}

// NewClient builds a client against the given API base URL.
func NewClient(baseURL, clientID, secret string) *Client {
	return &Client{
		baseURL:    baseURL,
		clientID:   clientID,
		secret:     secret,
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

// Authenticate exchanges the vendor credentials for an access token.
func (c *Client) Authenticate(ctx context.Context) error {
	authURL := c.baseURL + "/auth/vendor"

	body, err := json.Marshal(map[string]string{"clientId": c.clientID, "secret": c.secret})
	if err != nil {
		return fmt.Errorf("marshal auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute auth request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if !isSuccess(resp.StatusCode) {
		payload, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Method:     http.MethodPost,
			Path:       "/auth/vendor",
			Body:       string(payload),
			TraceID:    resp.Header.Get("frontegg-trace-id"),
		}
	}

	var decoded authResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return fmt.Errorf("decode auth response: %w", err)
	}

	c.mu.Lock()
	c.accessToken = decoded.Token
	c.tokenExpiry = time.Now().Add(time.Duration(decoded.ExpiresIn)*time.Second - tokenRefreshBuffer)
	c.mu.Unlock()

	tflog.Info(ctx, "authenticated with Frontegg API", map[string]interface{}{
		"expires_in_seconds": decoded.ExpiresIn,
	})
	return nil
}

func (c *Client) accessTokenValue(ctx context.Context) (string, error) {
	c.mu.RLock()
	token, expiry := c.accessToken, c.tokenExpiry
	c.mu.RUnlock()

	if token != "" && time.Now().Before(expiry) {
		return token, nil
	}
	if err := c.Authenticate(ctx); err != nil {
		return "", err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken, nil
}

// do issues an authenticated request and decodes a successful JSON body into out when out is non-nil.
func (c *Client) do(ctx context.Context, method, path string, body, out interface{}) error {
	token, err := c.accessTokenValue(ctx)
	if err != nil {
		return err
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	tflog.Debug(ctx, "calling Frontegg API", map[string]interface{}{"method": method, "path": path})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	payload, readErr := io.ReadAll(resp.Body)

	if !isSuccess(resp.StatusCode) {
		return &APIError{
			StatusCode: resp.StatusCode,
			Method:     method,
			Path:       path,
			Body:       string(payload),
			TraceID:    resp.Header.Get("frontegg-trace-id"),
		}
	}
	if readErr != nil {
		return fmt.Errorf("read response body: %w", readErr)
	}

	if out == nil || len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode response from %s %s: %w", method, path, err)
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) post(ctx context.Context, path string, body, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

func (c *Client) patch(ctx context.Context, path string, body, out interface{}) error {
	return c.do(ctx, http.MethodPatch, path, body, out)
}

func (c *Client) put(ctx context.Context, path string, body, out interface{}) error {
	return c.do(ctx, http.MethodPut, path, body, out)
}

func (c *Client) delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func isSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// withQuery appends the non-empty parameters to path as a query string.
func withQuery(path string, params map[string]string) string {
	values := url.Values{}
	for key, value := range params {
		if value != "" {
			values.Set(key, value)
		}
	}
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}
