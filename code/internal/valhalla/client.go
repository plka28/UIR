package valhalla

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the expected local Valhalla HTTP service address.
	DefaultBaseURL = "http://localhost:8002"

	defaultTimeout = 30 * time.Second
)

// ErrServiceUnavailable marks network-level failures or HTTP 503 responses.
var ErrServiceUnavailable = errors.New("valhalla service unavailable")

// Config controls how the Valhalla client connects to the service.
type Config struct {
	BaseURL    string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// Client is a small HTTP client for the local Valhalla service.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

// HTTPError describes a non-success response from Valhalla.
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (err *HTTPError) Error() string {
	if err.Body == "" {
		return fmt.Sprintf("valhalla HTTP error: %s", err.Status)
	}
	return fmt.Sprintf("valhalla HTTP error: %s: %s", err.Status, err.Body)
}

// NewClient creates a Valhalla client. Empty BaseURL defaults to http://localhost:8002.
func NewClient(config Config) (*Client, error) {
	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Valhalla base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("parse Valhalla base URL: absolute URL required: %q", baseURL)
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		timeout := config.Timeout
		if timeout == 0 {
			timeout = defaultTimeout
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	return &Client{
		baseURL:    parsed,
		httpClient: httpClient,
	}, nil
}

func (client *Client) postJSON(ctx context.Context, endpoint string, request any, response any) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode Valhalla request: %w", err)
	}

	endpointURL := client.resolveEndpoint(endpoint)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build Valhalla request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")

	httpResponse, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrServiceUnavailable, err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		httpErr := &HTTPError{
			StatusCode: httpResponse.StatusCode,
			Status:     httpResponse.Status,
			Body:       readErrorBody(httpResponse.Body),
		}
		if httpResponse.StatusCode == http.StatusServiceUnavailable {
			return fmt.Errorf("%w: %v", ErrServiceUnavailable, httpErr)
		}
		return httpErr
	}

	decoder := json.NewDecoder(httpResponse.Body)
	if err := decoder.Decode(response); err != nil {
		return fmt.Errorf("decode Valhalla response JSON: %w", err)
	}
	return nil
}

func (client *Client) resolveEndpoint(endpoint string) string {
	resolved := *client.baseURL
	basePath := strings.TrimRight(resolved.Path, "/")
	endpointPath := strings.TrimLeft(endpoint, "/")
	resolved.Path = basePath + "/" + endpointPath
	return resolved.String()
}

func readErrorBody(body io.Reader) string {
	const maxErrorBodyBytes = 1024

	limited, err := io.ReadAll(io.LimitReader(body, maxErrorBodyBytes))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(limited))
}
