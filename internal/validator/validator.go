package validator

import (
	"fmt"
	"net/url"
	"strings"

	"aurl/internal/spec"
)

// ValidationError represents a validation failure with a clear message
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("[%s] %s", e.Field, e.Message)
	}
	return e.Message
}

// RequestValidation combines extracted params from the URL and a body into a map
// for validation against an endpoint's spec
type RequestValidation struct {
	PathParams   map[string]string
	QueryParams  map[string]string
	HeaderParams map[string]string
	CookieParams map[string]string
	Body         string
}

// ValidateEndpoint checks a request against an endpoint's parameter requirements
func ValidateEndpoint(endpoint *spec.Endpoint, req *RequestValidation) error {
	if endpoint == nil {
		return &ValidationError{Message: "endpoint not found in spec"}
	}

	var failures []string

	// Validate each parameter defined in the spec
	for _, param := range endpoint.Parameters {
		var provided string
		var paramLoc string

		switch param.In {
		case "path":
			provided = req.PathParams[param.Name]
			paramLoc = "path"
		case "query":
			provided = req.QueryParams[param.Name]
			paramLoc = "query"
		case "header":
			provided = req.HeaderParams[param.Name]
			paramLoc = "header"
		case "cookie":
			provided = req.CookieParams[param.Name]
			paramLoc = "cookie"
		}

		// Check required
		if param.Required && provided == "" {
			failures = append(failures, fmt.Sprintf("parameter %q (%s) is required but not provided", param.Name, paramLoc))
			continue
		}

		// Check enum if provided and has values
		if provided != "" && len(param.Enum) > 0 {
			found := false
			for _, v := range param.Enum {
				if v == provided {
					found = true
					break
				}
			}
			if !found {
				failures = append(failures, fmt.Sprintf("parameter %q value %q is not one of the allowed values: %v", param.Name, provided, param.Enum))
			}
		}
	}

	if len(failures) > 0 {
		return &ValidationError{
			Message: strings.Join(failures, "; "),
		}
	}

	return nil
}

// ExtractParamsFromURL parses a full URL and splits params into path/query
func ExtractParamsFromURL(rawURL string) (path string, queryParams, pathParams map[string]string) {
	queryParams = make(map[string]string)
	pathParams = make(map[string]string)

	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		path = rawURL
		return
	}

	path = u.Path
	for k, v := range u.Query() {
		if len(v) > 0 {
			queryParams[k] = v[0]
		}
	}
	return
}

// AutoDetectAuth inspects endpoint + spec-level security and returns an auth header injection
// Returns (headerKey, headerValue, authType)
func AutoDetectAuth(endpoint *spec.Endpoint, specSchemes []spec.SecurityScheme) (string, string, string) {
	// Check endpoint-level security first
	if endpoint != nil && len(endpoint.Security) > 0 {
		for _, ss := range endpoint.Security {
			if h, v, t := schemeToAuth(ss); h != "" {
				return h, v, t
			}
		}
	}

	// Fall back to spec-level schemes
	for _, ss := range specSchemes {
		if h, v, t := schemeToAuth(ss); h != "" {
			return h, v, t
		}
	}

	return "", "", ""
}

func schemeToAuth(ss spec.SecurityScheme) (string, string, string) {
	switch ss.Type {
	case "apiKey":
		// API key in header
		if ss.In == "header" && ss.Name != "" {
			return ss.Name, "<API_KEY>", "api_key"
		}
		// API key in query
		if ss.In == "query" && ss.Name != "" {
			return "", "", ""
		}
	case "http":
		// Bearer token
		if ss.Scheme == "bearer" {
			return "Authorization", "Bearer <TOKEN>", "bearer"
		}
		// Basic auth
		if ss.Scheme == "basic" {
			return "Authorization", "Basic <CREDENTIALS>", "basic"
		}
	case "oauth2":
		return "Authorization", "Bearer <OAUTH_TOKEN>", "bearer"
	}
	return "", "", ""
}
