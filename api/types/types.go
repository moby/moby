package types

// ContainerCreateResponse contains the information returned to a client on the
// creation of a new container.
type ContainerCreateResponse struct {
	// ID is the ID of the created container.
	ID string `json:"Id"`

	// Warnings are any warnings encountered during the creation of the container.
	Warnings []string `json:"Warnings"`
}
