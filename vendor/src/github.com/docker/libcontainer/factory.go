package libcontainer

import (
	"github.com/docker/libcontainer/configs"
)

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
	// errors:
	// IdInUse - id is already in use by a container
	// InvalidIdFormat - id has incorrect format
	// ConfigInvalid - config is invalid
	// Systemerror - System error
	//
	// On error, any partially created container parts are cleaned up (the operation is atomic).
	Create(id string, config *configs.Config) (Container, error)

	// Load takes an ID for an existing container and returns the container information
	// from the state.  This presents a read only view of the container.
	//
	// errors:
	// Path does not exist
	// Container is stopped
	// System error
	Load(id string) (Container, error)

	// StartInitialization is an internal API to libcontainer used during the rexec of the
	// container.  pipefd is the fd to the child end of the pipe used to syncronize the
	// parent and child process providing state and configuration to the child process and
	// returning any errors during the init of the container
	//
	// Errors:
	// pipe connection error
	// system error
	StartInitialization(pipefd uintptr) error

	// Type returns info string about factory type (e.g. lxc, libcontainer...)
	Type() string
}
