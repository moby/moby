//go:build linux

package bridge

import (
	"errors"

	"github.com/moby/moby/v2/errdefs"
)

// errInvalidGateway is returned when the user provided default gateway (v4/v6) is not valid.
var errInvalidGateway = errdefs.InvalidParameter(errors.New("default gateway ip must be part of the network"))

// invalidNetworkIDError is returned when the passed
// network id for an existing network is not a known id.
type invalidNetworkIDError string

func (e invalidNetworkIDError) Error() string {
	return "invalid network id " + string(e)
}

// NotFound denotes the type of this error
func (e invalidNetworkIDError) NotFound() {}

// invalidEndpointIDError is returned when the passed
// endpoint id is not valid.
type invalidEndpointIDError string

func (e invalidEndpointIDError) Error() string {
	return "invalid endpoint id: " + string(e)
}

// InvalidParameter denotes the type of this error
func (e invalidEndpointIDError) InvalidParameter() {}

// endpointNotFoundError is returned when the no endpoint
// with the passed endpoint id is found.
type endpointNotFoundError string

func (e endpointNotFoundError) Error() string {
	return "endpoint not found: " + string(e)
}

// NotFound denotes the type of this error
func (e endpointNotFoundError) NotFound() {}
