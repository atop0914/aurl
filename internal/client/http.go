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
	// Parse the URL to extract query parameters
	parsedURL, err := url.ParseRequestURI(req.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Build the full URL
	fullURL := parsedURL.String()

	// Determine body reader
	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	// Create the HTTP request
	httpReq, err := http.NewRequest(req.Method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range req.Header {
		httpReq.Header.Set(key, value)
	}

	// Set Content-Type for requests with body
	if req.Body != "" {
		if httpReq.Header.Get("Content-Type") == "" {
			httpReq.Header.Set("Content-Type", "application/json")
		}
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

// ParseURL extracts path and query parameters from a URL string
func ParseURL(rawURL string) (path string, queryParams url.Values) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, url.Values{}
	}
	return parsed.Path, parsed.Query()
}

// BuildURL constructs a full URL from baseURL and path
// If path contains query parameters, they are preserved
func BuildURL(baseURL, path string) string {
	// Check if path has query parameters
	if strings.Contains(path, "?") {
		// Parse the path to extract query
		parsed, err := url.Parse(path)
		if err == nil {
			// Build base + path + query
			baseURL = strings.TrimSuffix(baseURL, "/")
			path = strings.TrimPrefix(parsed.Path, "/")
			result := baseURL + "/" + path
			if parsed.RawQuery != "" {
				result += "?" + parsed.RawQuery
			}
			return result
		}
	}

	// Simple case: no query params
	baseURL = strings.TrimSuffix(baseURL, "/")
	path = strings.TrimPrefix(path, "/")
	return baseURL + "/" + path
}

// AddQueryParams adds query parameters to a URL string
func AddQueryParams(rawURL string, params map[string]string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsed.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
