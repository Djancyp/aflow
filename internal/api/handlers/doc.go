// Package handlers contains all HTTP request handlers for the aflow API.
package handlers

// ProblemDetail is an RFC 7807 problem details error response.
//
//	@Description RFC 7807 error envelope returned on all 4xx/5xx responses.
type ProblemDetail struct {
	Type   string `json:"type"             example:"https://aflow.dev/errors/not-found"`
	Title  string `json:"title"            example:"Not Found"`
	Status int    `json:"status"           example:"404"`
	Detail string `json:"detail,omitempty" example:"Workflow abc was not found"`
}

// ListMeta is embedded in paginated list responses.
type ListMeta struct {
	Count int `json:"count" example:"5"`
}
