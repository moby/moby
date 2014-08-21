package libcontainer

type Factory interface {
	// Creates a new container in the given path. A unique ID is generated for the container and
	// starts the initial process inside the container.
	//
	// Returns the new container with a running process.
	//
	// Errors:
	// Path already exists
	// Config or initialConfig is invalid
	// System error
	//
	// On error, any partially created container parts are cleaned up (the operation is atomic).
	Create(path string, config *Config) (Container, error)

	// Load takes the path for an existing container and reconstructs the container
	// from the state.
	//
	// Errors:
	// Path does not exist
	// Container is stopped
	// System error
	Load(path string) (Container, error)
}
