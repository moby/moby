// Code generated from OpenAPI definition. DO NOT EDIT.

package container

// CreateResponse OK response to ContainerCreate operation
type CreateResponse struct {
	// The ID of the created container
	// Example: ede54ee1afda366ab42f824e8a5ffd195155d853ceaec74a927f249ea270c743
	// Required: true
	ID string `json:"Id"`

	// Warnings encountered when creating the container
	// Required: true
	Warnings []string `json:"Warnings"`
}
