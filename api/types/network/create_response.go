// Code generated from OpenAPI definition. DO NOT EDIT.

package network

// CreateResponse OK response to NetworkCreate operation
type CreateResponse struct {
	// The ID of the created network.
	// Example: b5c4fc71e8022147cd25de22b22173de4e3b170134117172eb595cb91b4e7e5d
	// Required: true
	ID string `json:"Id"`

	// Warnings encountered when creating the container
	// Required: true
	Warning string `json:"Warning"`
}
