package openapi

// FullEndpoint contains all data extracted from the OpenAPI spec for a
// single operation, without truncation. This is the rich representation
// stored in the SQLite FTS database.
type FullEndpoint struct {
	Path        string   `json:"path"`
	Method      string   `json:"method"`
	OperationID string   `json:"operationId,omitempty"`
	Summary     string   `json:"summary"`
	Description string   `json:"description,omitempty"`
	Resource    string   `json:"resource,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	Tags        []string `json:"tags,omitempty"`

	// Documentation
	DocURL          string `json:"docUrl,omitempty"`
	PathDescription string `json:"pathDescription,omitempty"`

	// Deprecation
	Deprecated             bool   `json:"deprecated,omitempty"`
	DeprecationDate        string `json:"deprecationDate,omitempty"`
	DeprecationRemovalDate string `json:"deprecationRemovalDate,omitempty"`
	DeprecationDescription string `json:"deprecationDescription,omitempty"`

	// Operation metadata
	OperationType string `json:"operationType,omitempty"` // operation, action, function
	Pageable      bool   `json:"pageable,omitempty"`

	// Parameters
	Parameters []Parameter `json:"parameters,omitempty"`

	// Request/Response schema references
	RequestBodyRef  string `json:"requestBodyRef,omitempty"`
	RequestBodyDesc string `json:"requestBodyDesc,omitempty"`
	ResponseRef     string `json:"responseRef,omitempty"`
}

// Parameter represents an API parameter extracted from the OpenAPI spec.
type Parameter struct {
	Name string `json:"name"`
	In   string `json:"in"` // query, header, path
}
