package types

// ContainerCreateResponse contains the information returned to a client on the
// creation of a new container.
type ContainerCreateResponse struct {
	// ID is the ID of the created container.
	ID string `json:"Id"`

	// Warnings are any warnings encountered during the creation of the container.
	Warnings []string `json:"Warnings"`
}

// POST /containers/{name:.*}/exec
type ContainerExecCreateResponse struct {
	// ID is the exec ID.
	ID string `json:"Id"`

	// Warnings are any warnings encountered during the execution of the command.
	Warnings []string `json:"Warnings"`
}
