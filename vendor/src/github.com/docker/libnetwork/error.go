package libnetwork

import (
	"errors"
	"fmt"
)

var (
	// ErrNoSuchNetwork is returned when a network query finds no result
	ErrNoSuchNetwork = errors.New("network not found")
	// ErrNoSuchEndpoint is returned when a endpoint query finds no result
	ErrNoSuchEndpoint = errors.New("endpoint not found")
	// ErrNilNetworkDriver is returned if a nil network driver
	// is passed to NewNetwork api.
	ErrNilNetworkDriver = errors.New("nil NetworkDriver instance")
	// ErrInvalidNetworkDriver is returned if an invalid driver
	// instance is passed.
	ErrInvalidNetworkDriver = errors.New("invalid driver bound to network")
	// ErrInvalidJoin is returned if a join is attempted on an endpoint
	// which already has a container joined.
	ErrInvalidJoin = errors.New("a container has already joined the endpoint")
	// ErrNoContainer is returned when the endpoint has no container
	// attached to it.
	ErrNoContainer = errors.New("no container attached to the endpoint")
	// ErrInvalidID is returned when a query-by-id method is being invoked
	// with an empty id parameter
	ErrInvalidID = errors.New("invalid ID")
	// ErrInvalidName is returned when a query-by-name or resource create method is
	// invoked with an empty name parameter
	ErrInvalidName = errors.New("invalid Name")
)

// NetworkTypeError type is returned when the network type string is not
// known to libnetwork.
type NetworkTypeError string

func (nt NetworkTypeError) Error() string {
	return fmt.Sprintf("unknown driver %q", string(nt))
}

// NetworkNameError is returned when a network with the same name already exists.
type NetworkNameError string

func (name NetworkNameError) Error() string {
	return fmt.Sprintf("network with name %s already exists", string(name))
}

// UnknownNetworkError is returned when libnetwork could not find in it's database
// a network with the same name and id.
type UnknownNetworkError struct {
	name string
	id   string
}

func (une *UnknownNetworkError) Error() string {
	return fmt.Sprintf("unknown network %s id %s", une.name, une.id)
}

// ActiveEndpointsError is returned when a network is deleted which has active
// endpoints in it.
type ActiveEndpointsError struct {
	name string
	id   string
}

func (aee *ActiveEndpointsError) Error() string {
	return fmt.Sprintf("network with name %s id %s has active endpoints", aee.name, aee.id)
}

// UnknownEndpointError is returned when libnetwork could not find in it's database
// an endpoint with the same name and id.
type UnknownEndpointError struct {
	name string
	id   string
}

func (uee *UnknownEndpointError) Error() string {
	return fmt.Sprintf("unknown endpoint %s id %s", uee.name, uee.id)
}

// ActiveContainerError is returned when an endpoint is deleted which has active
// containers attached to it.
type ActiveContainerError struct {
	name string
	id   string
}

func (ace *ActiveContainerError) Error() string {
	return fmt.Sprintf("endpoint with name %s id %s has active containers", ace.name, ace.id)
}

// InvalidContainerIDError is returned when an invalid container id is passed
// in Join/Leave
type InvalidContainerIDError string

func (id InvalidContainerIDError) Error() string {
	return fmt.Sprintf("invalid container id %s", string(id))
}
