package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"aurl/internal/spec"
)

// OpenAPI3 parses OpenAPI 3.x JSON specifications
type OpenAPI3 struct{}

// NewOpenAPI3 creates a new OpenAPI3 parser
func NewOpenAPI3() *OpenAPI3 {
	return &OpenAPI3{}
}

// RawOpenAPI represents the raw structure of an OpenAPI 3.x spec
type RawOpenAPI struct {
	OpenAPI string `json:"openapi"`
	Info    struct {
		Title       string `json:"title"`
		Version     string `json:"version"`
		Description string `json:"description"`
	} `json:"info"`
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths map[string]json.RawMessage `json:"paths"`
}

// Parse fetches and parses an OpenAPI 3.x spec from a URL or file
func (p *OpenAPI3) Parse(specRef string) (*spec.ParsedSpec, error) {
	var body []byte
	var err error
	var specURL *url.URL

	if u, parseErr := url.ParseRequestURI(specRef); parseErr == nil && (u.Scheme == "http" || u.Scheme == "https") {
		// It's a URL
		specURL = u
		resp, err := http.Get(specRef)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch spec: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch spec: HTTP %d", resp.StatusCode)
		}

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read spec body: %w", err)
		}
	} else {
		// It's a file path
		body, err = os.ReadFile(specRef)
		if err != nil {
			return nil, fmt.Errorf("failed to read spec file: %w", err)
		}
	}

	var raw RawOpenAPI
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Determine base URL
	baseURL := ""
	if len(raw.Servers) > 0 {
		serverURL := raw.Servers[0].URL
		// Check if server URL is relative (starts with /)
		if strings.HasPrefix(serverURL, "/") && specURL != nil {
			// Resolve relative server URL against the spec URL's host
			baseURL = specURL.Scheme + "://" + specURL.Host + serverURL
		} else {
			baseURL = serverURL
		}
	}

	parsed := &spec.ParsedSpec{
		Title:     raw.Info.Title,
		Version:   raw.Info.Version,
		BaseURL:   baseURL,
		Endpoints: []spec.Endpoint{},
		TagGroups: make(map[string][]spec.Endpoint),
	}

	// Parse paths
	for path, pathRaw := range raw.Paths {
		var pathItem RawPathItem
		if err := json.Unmarshal(pathRaw, &pathItem); err != nil {
			continue
		}

		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
		for _, method := range methods {
			var operation *RawOperation
			switch method {
			case "GET":
				operation = pathItem.Get
			case "POST":
				operation = pathItem.Post
			case "PUT":
				operation = pathItem.Put
			case "DELETE":
				operation = pathItem.Delete
			case "PATCH":
				operation = pathItem.Patch
			case "OPTIONS":
				operation = pathItem.Options
			case "HEAD":
				operation = pathItem.Head
			}

			if operation == nil {
				continue
			}

			// Get tags, default to "default" if none
			tags := operation.Tags
			if len(tags) == 0 {
				tags = []string{"default"}
			}

			summary := operation.Summary
			if summary == "" {
				summary = fmt.Sprintf("%s %s", strings.ToUpper(method), path)
			}

			endpoint := spec.Endpoint{
				Method:  strings.ToUpper(method),
				Path:    path,
				Summary: summary,
				Tags:    tags,
			}

			parsed.Endpoints = append(parsed.Endpoints, endpoint)

			// Group by tag
			for _, tag := range tags {
				parsed.TagGroups[tag] = append(parsed.TagGroups[tag], endpoint)
			}
		}
	}

	return parsed, nil
}

// RawPathItem represents a path item in OpenAPI spec
type RawPathItem struct {
	Ref     string          `json:"$ref,omitempty"`
	Get     *RawOperation   `json:"get,omitempty"`
	Post    *RawOperation   `json:"post,omitempty"`
	Put     *RawOperation   `json:"put,omitempty"`
	Delete  *RawOperation   `json:"delete,omitempty"`
	Patch   *RawOperation   `json:"patch,omitempty"`
	Options *RawOperation   `json:"options,omitempty"`
	Head    *RawOperation   `json:"head,omitempty"`
}

// RawOperation represents an operation in OpenAPI spec
type RawOperation struct {
	Tags        []string `json:"tags,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	OperationID string   `json:"operationId,omitempty"`
	Description string   `json:"description,omitempty"`
	Parameters  []struct {
		Name     string `json:"name"`
		In       string `json:"in"`
		Required bool   `json:"required"`
		Type     string `json:"type"`
		Enum     []string `json:"enum,omitempty"`
		Schema   struct {
			Type string `json:"type"`
		} `json:"schema,omitempty"`
	} `json:"parameters,omitempty"`
	RequestBody *struct {
		Content map[string]struct {
			Schema *json.RawMessage `json:"schema,omitempty"`
		} `json:"content,omitempty"`
	} `json:"requestBody,omitempty"`
	Responses map[string]struct {
		Description string `json:"description"`
	} `json:"responses,omitempty"`
}

// GetEndpoints returns a flat list of all endpoints (convenience method)
func (p *OpenAPI3) GetEndpoints(parsed *spec.ParsedSpec) []spec.Endpoint {
	return parsed.Endpoints
}
