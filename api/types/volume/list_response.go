// Code generated from OpenAPI definition. DO NOT EDIT.

package volume

// ListResponse Volume list response
type ListResponse struct {
	// List of volumes
	Volumes []Volume `json:"Volumes,omitempty"`

	// Warnings that occurred when fetching the list of volumes.
	//
	Warnings []string `json:"Warnings,omitempty"`
}
