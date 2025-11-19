// Code generated from OpenAPI definition. DO NOT EDIT.

package container

// UpdateResponse Response for a successful container-update.
type UpdateResponse struct {
	// Warnings encountered when updating the container.
	// Example: Published ports are discarded when using host network mode
	Warnings []string `json:"Warnings,omitempty"`
}
