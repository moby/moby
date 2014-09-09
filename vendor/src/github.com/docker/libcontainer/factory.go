package libcontainer

type Factory interface {

	// Creates a new container with the given id and starts the initial process inside it.
	// id must be a string containing only letters, digits and underscores and must contain
	// between 1 and 1024 characters, inclusive.
	//
	// The id must not already be in use by an existing container. Containers created using
	// a factory with the same path (and file system) must have distinct ids.
	//
	// Returns the new container with a running process.
	//
	// Errors:
	// id is already in use by a container
	// id has incorrect format
	// config is invalid
	// System error
	//
	// On error, any partially created container parts are cleaned up (the operation is atomic).
	Create(id string, config *Config) (Container, error)

	// Load takes an ID for an existing container and reconstructs the container
	// from the state.
	//
	// Errors:
	// Path does not exist
	// Container is stopped
	// System error
	Load(id string) (Container, error)
}
