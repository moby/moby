package libnetwork

import (
	"fmt"
)

// ErrNoSuchNetwork is returned when a network query finds no result
type ErrNoSuchNetwork string

func (nsn ErrNoSuchNetwork) Error() string {
	return fmt.Sprintf("network %s not found", string(nsn))
}

// NotFound denotes the type of this error
func (nsn ErrNoSuchNetwork) NotFound() {}

// ErrNoSuchEndpoint is returned when an endpoint query finds no result
type ErrNoSuchEndpoint string

func (nse ErrNoSuchEndpoint) Error() string {
	return fmt.Sprintf("endpoint %s not found", string(nse))
}

// NotFound denotes the type of this error
func (nse ErrNoSuchEndpoint) NotFound() {}

// ErrInvalidID is returned when a query-by-id method is being invoked
// with an empty id parameter
type ErrInvalidID string

func (ii ErrInvalidID) Error() string {
	return fmt.Sprintf("invalid id: %s", string(ii))
}

// BadRequest denotes the type of this error
func (ii ErrInvalidID) BadRequest() {}

// ErrInvalidName is returned when a query-by-name or resource create method is
// invoked with an empty name parameter
type ErrInvalidName string

func (in ErrInvalidName) Error() string {
	return fmt.Sprintf("invalid name: %s", string(in))
}

// BadRequest denotes the type of this error
func (in ErrInvalidName) BadRequest() {}

// NetworkNameError is returned when a network with the same name already exists.
type NetworkNameError string

func (nnr NetworkNameError) Error() string {
	return fmt.Sprintf("network with name %s already exists", string(nnr))
}

// Conflict denotes the type of this error
func (nnr NetworkNameError) Conflict() {}

// UnknownNetworkError is returned when libnetwork could not find in its database
// a network with the same name and id.
type UnknownNetworkError struct {
	name string
	id   string
}

func (une *UnknownNetworkError) Error() string {
	return fmt.Sprintf("unknown network %s id %s", une.name, une.id)
}

// NotFound denotes the type of this error
func (une *UnknownNetworkError) NotFound() {}

// ActiveEndpointsError is returned when a network is deleted which has active
// endpoints in it.
type ActiveEndpointsError struct {
	name string
	id   string
}

func (aee *ActiveEndpointsError) Error() string {
	return fmt.Sprintf("network %s id %s has active endpoints", aee.name, aee.id)
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
