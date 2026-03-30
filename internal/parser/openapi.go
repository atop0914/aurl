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
	Paths         map[string]json.RawMessage   `json:"paths"`
	Components    RawComponents                 `json:"components,omitempty"`
	Security      []map[string][]string        `json:"security,omitempty"`
}

type RawComponents struct {
	SecuritySchemes map[string]json.RawMessage `json:"securitySchemes,omitempty"`
}

// RawPathItem represents a path item in OpenAPI spec
type RawPathItem struct {
	Ref      string          `json:"$ref,omitempty"`
	Get     *RawOperation   `json:"get,omitempty"`
	Post    *RawOperation   `json:"post,omitempty"`
	Put     *RawOperation   `json:"put,omitempty"`
	Delete  *RawOperation   `json:"delete,omitempty"`
	Patch   *RawOperation   `json:"patch,omitempty"`
	Options *RawOperation   `json:"options,omitempty"`
	Head    *RawOperation   `json:"head,omitempty"`
	Parameters []RawParameter `json:"parameters,omitempty"`
}

// RawOperation represents an operation in OpenAPI spec
type RawOperation struct {
	Tags         []string              `json:"tags,omitempty"`
	Summary      string                `json:"summary,omitempty"`
	OperationID  string                `json:"operationId,omitempty"`
	Description  string                `json:"description,omitempty"`
	Parameters   []RawParameter        `json:"parameters,omitempty"`
	RequestBody  *RawRequestBody       `json:"requestBody,omitempty"`
	Responses    map[string]RawResponse `json:"responses,omitempty"`
	Security     []map[string][]string  `json:"security,omitempty"`
}

type RawParameter struct {
	Name        string   `json:"name"`
	In         string   `json:"in"`
	Required   bool     `json:"required"`
	Type       string   `json:"type,omitempty"`
	Enum       []string `json:"enum,omitempty"`
	Description string  `json:"description,omitempty"`
	Schema      RawSchema `json:"schema,omitempty"`
}

type RawSchema struct {
	Type        string   `json:"type,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Format      string   `json:"format,omitempty"`
	Description string   `json:"description,omitempty"`
}

type RawRequestBody struct {
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Content     map[string]struct {
		Schema *json.RawMessage `json:"schema,omitempty"`
	} `json:"content,omitempty"`
}

type RawResponse struct {
	Description string `json:"description"`
}

// RawSecurityScheme represents a raw security scheme
type RawSecurityScheme struct {
	Type        string `json:"type"`
	Scheme      string `json:"scheme,omitempty"`
	Name        string `json:"name,omitempty"`
	In          string `json:"in,omitempty"`
	Description string `json:"description,omitempty"`
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
		Title:            raw.Info.Title,
		Version:          raw.Info.Version,
		BaseURL:          baseURL,
		Endpoints:       []spec.Endpoint{},
		TagGroups:        make(map[string][]spec.Endpoint),
		SecuritySchemes:  p.parseSecuritySchemes(raw.Components.SecuritySchemes),
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

			// Merge path-level and operation-level parameters
			allParams := p.mergeParameters(pathItem.Parameters, operation.Parameters)

			// Convert to spec.Parameter
			params := make([]spec.Parameter, 0, len(allParams))
			for _, rp := range allParams {
				paramType := rp.Type
				enumVals := rp.Enum
				// Try to get type/enum from schema if not inlined
				if paramType == "" && rp.Schema.Type != "" {
					paramType = rp.Schema.Type
				}
				if len(enumVals) == 0 && len(rp.Schema.Enum) > 0 {
					enumVals = rp.Schema.Enum
				}
				params = append(params, spec.Parameter{
					Name:        rp.Name,
					In:          rp.In,
					Required:    rp.Required,
					Type:        paramType,
					Enum:        enumVals,
					Description: rp.Description,
				})
			}

			// Determine security for this operation
			// Operation-level security overrides spec-level
			security := p.resolveSecurity(operation.Security, raw.Security, parsed.SecuritySchemes)

			endpoint := spec.Endpoint{
				Method:     strings.ToUpper(method),
				Path:       path,
				Summary:    summary,
				Tags:       tags,
				Parameters: params,
				Security:   security,
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

// mergeParameters combines path-level and operation-level parameters
// Operation-level params override path-level params with the same name
func (p *OpenAPI3) mergeParameters(pathParams, opParams []RawParameter) []RawParameter {
	// Index path params by name+in
	merged := make(map[string]RawParameter)
	for _, pp := range pathParams {
		key := pp.Name + ":" + pp.In
		merged[key] = pp
	}
	// Operation params override
	for _, op := range opParams {
		key := op.Name + ":" + op.In
		merged[key] = op
	}

	// Convert back to slice
	result := make([]RawParameter, 0, len(merged))
	for _, v := range merged {
		result = append(result, v)
	}
	return result
}

// parseSecuritySchemes converts raw security schemes to spec.SecurityScheme
func (p *OpenAPI3) parseSecuritySchemes(raw map[string]json.RawMessage) []spec.SecurityScheme {
	if raw == nil {
		return nil
	}

	var schemes []spec.SecurityScheme
	for name, rawScheme := range raw {
		var rss RawSecurityScheme
		if err := json.Unmarshal(rawScheme, &rss); err != nil {
			continue
		}
		ss := spec.SecurityScheme{
			Name:        name,
			Type:        rss.Type,
			Scheme:      rss.Scheme,
			In:          rss.In,
			Description: rss.Description,
		}
		schemes = append(schemes, ss)
	}
	return schemes
}

// resolveSecurity determines effective security for an operation
func (p *OpenAPI3) resolveSecurity(opSecurity []map[string][]string, globalSecurity []map[string][]string, schemes []spec.SecurityScheme) []spec.SecurityScheme {
	// Build a map of scheme name -> spec.SecurityScheme for quick lookup
	schemeMap := make(map[string]spec.SecurityScheme)
	for _, s := range schemes {
		schemeMap[s.Name] = s
	}

	// Use operation-level security if specified
	securityList := opSecurity
	if len(securityList) == 0 {
		securityList = globalSecurity
	}

	if len(securityList) == 0 {
		return nil
	}

	var result []spec.SecurityScheme
	for _, sec := range securityList {
		// sec is map[s_scheme_name] -> []string (scopes for oauth2)
		for schemeName := range sec {
			if ss, ok := schemeMap[schemeName]; ok {
				result = append(result, ss)
			}
		}
	}
	return result
}

// FindEndpoint locates an endpoint by method and path
func (p *OpenAPI3) FindEndpoint(parsed *spec.ParsedSpec, method, path string) *spec.Endpoint {
	method = strings.ToUpper(method)
	for i := range parsed.Endpoints {
		ep := &parsed.Endpoints[i]
		if ep.Method == method && ep.Path == path {
			return ep
		}
	}
	return nil
}

// GetEndpoints returns a flat list of all endpoints (convenience method)
func (p *OpenAPI3) GetEndpoints(parsed *spec.ParsedSpec) []spec.Endpoint {
	return parsed.Endpoints
}
