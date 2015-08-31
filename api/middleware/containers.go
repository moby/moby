package middleware

import (
	"fmt"
	"net/http"

	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/version"
)

// errMissingParameter is the error to return when a request variable is missing.
type errMissingParameter struct {
	// name is the name of the expected variable
	name string
}

// Error prints a pretty version if the errMissingParameter error.
// Satifies the Error interface.
func (e errMissingParameter) Error() string {
	return fmt.Sprintf("missing parameter: %s", e.name)
}

// ContainerRequest is a representation of a request to manipulate a container.
// It includes the container to work with, the api version, the original request, and the request variables.
type ContainerRequest struct {
	// Container is the container we're going to work with resolved from the request
	Container *daemon.Container
	// Version is the api version requested
	Version version.Version
	// Vars are the request variables extracted from the path
	vars map[string]string
	// Request is the original request
	*http.Request
}

// NewContainerRequest generates a new struct to serve a request to container operations.
func NewContainerRequest(r *http.Request, version version.Version, vars map[string]string) *ContainerRequest {
	return &ContainerRequest{
		Version: version,
		vars:    vars,
		Request: r,
	}
}

// GetVar retrieves variables from the request vars in a consistent way.
// If there are no variables or the variable doesn't exist, it returns an error.
func (c *ContainerRequest) GetVar(key string) (string, error) {
	if c.vars == nil {
		return "", errMissingParameter{key}
	}

	v, ok := c.vars[key]
	if !ok {
		return "", errMissingParameter{key}
	}

	return v, nil
}

// ContainerHandler represents a function to handle a container operation.
type ContainerHandler func(w http.ResponseWriter, r *ContainerRequest) error
