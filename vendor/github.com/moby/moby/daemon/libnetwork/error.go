package libnetwork

import (
	"fmt"
	"strings"
)

// ErrNoSuchNetwork is returned when a network query finds no result
type ErrNoSuchNetwork string

func (nsn ErrNoSuchNetwork) Error() string {
	return fmt.Sprintf("network %s not found", string(nsn))
}

// NotFound denotes the type of this error
func (nsn ErrNoSuchNetwork) NotFound() {}

// NetworkNameError is returned when a network with the same name already exists.
type NetworkNameError string

func (nnr NetworkNameError) Error() string {
	return fmt.Sprintf("network with name %s already exists", string(nnr))
}

// Conflict denotes the type of this error
func (nnr NetworkNameError) Conflict() {}

// ActiveEndpointsError is returned when a network is deleted which has active
// endpoints in it.
type ActiveEndpointsError struct {
	name      string
	id        string
	endpoints []string
}

func (aee *ActiveEndpointsError) Error() string {
	return fmt.Sprintf("network %s has active endpoints (%s)", aee.name, strings.Join(aee.endpoints, ", "))
}

// Forbidden denotes the type of this error
func (aee *ActiveEndpointsError) Forbidden() {}

// ActiveContainerError is returned when an endpoint is deleted which has active
// containers attached to it.
type ActiveContainerError struct {
	name string
	id   string
}

func (ace *ActiveContainerError) Error() string {
	return fmt.Sprintf("endpoint with name %s id %s has active containers", ace.name, ace.id)
}

// Forbidden denotes the type of this error
func (ace *ActiveContainerError) Forbidden() {}

// ManagerRedirectError is returned when the request should be redirected to Manager
type ManagerRedirectError string

func (mr ManagerRedirectError) Error() string {
	return "Redirect the request to the manager"
}

// Maskable denotes the type of this error
func (mr ManagerRedirectError) Maskable() {}
