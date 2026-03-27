package spec

// Endpoint represents a parsed API endpoint
type Endpoint struct {
	Method  string   `json:"method"`
	Path    string   `json:"path"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

// Parameter represents a parsed parameter
type Parameter struct {
	Name     string   `json:"name"`
	In       string   `json:"in"` // "path", "query", "header", "cookie"
	Required bool     `json:"required"`
	Type     string   `json:"type"`
	Enum     []string `json:"enum,omitempty"`
}

// ParsedSpec holds all endpoints and metadata from an OpenAPI spec
type ParsedSpec struct {
	Title     string
	Version   string
	BaseURL   string
	Endpoints []Endpoint
	TagGroups map[string][]Endpoint
}
