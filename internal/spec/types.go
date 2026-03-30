package spec

// Endpoint represents a parsed API endpoint
type Endpoint struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Summary    string            `json:"summary"`
	Tags       []string          `json:"tags"`
	Parameters []Parameter       `json:"parameters,omitempty"`
	Security   []SecurityScheme `json:"security,omitempty"`
}

// Parameter represents a parsed parameter
type Parameter struct {
	Name     string   `json:"name"`
	In       string   `json:"in"` // "path", "query", "header", "cookie"
	Required bool     `json:"required"`
	Type     string   `json:"type"`
	Enum     []string `json:"enum,omitempty"`
	Description string `json:"description,omitempty"`
}

// SecurityScheme represents a security scheme from the spec
type SecurityScheme struct {
	Type        string `json:"type"` // "apiKey", "http", "oauth2"
	Scheme      string `json:"scheme,omitempty"` // "bearer" for http type
	Name        string `json:"name,omitempty"` // for apiKey type
	In          string `json:"in,omitempty"` // for apiKey type: "header", "query", "cookie"
	Description string `json:"description,omitempty"`
}

// ParsedSpec holds all endpoints and metadata from an OpenAPI spec
type ParsedSpec struct {
	Title          string
	Version        string
	BaseURL        string
	Endpoints      []Endpoint
	TagGroups      map[string][]Endpoint
	SecuritySchemes []SecurityScheme
}
