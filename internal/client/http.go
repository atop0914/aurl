package client

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// HTTPClient handles HTTP requests
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient creates a new HTTP client
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{},
	}
}

// Request represents an API request
type Request struct {
	Method string
	URL    string
	Body   string
	Header map[string]string
}

// Response represents an API response
type Response struct {
	StatusCode int
	Body       string
	Headers    http.Header
}

// Do executes an HTTP request and returns the response
func (c *HTTPClient) Do(req Request) (*Response, error) {
	// Parse the URL
	parsedURL, err := url.ParseRequestURI(req.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Build the full URL with path
	fullURL := parsedURL.String()

	// Create the HTTP request
	httpReq, err := http.NewRequest(req.Method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range req.Header {
		httpReq.Header.Set(key, value)
	}

	// Execute the request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       string(body),
		Headers:    resp.Header,
	}, nil
}

// BuildURL constructs a full URL from baseURL and path
func BuildURL(baseURL, path string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")
	path = strings.TrimPrefix(path, "/")
	return baseURL + "/" + path
}
