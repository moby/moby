//go:build linux

package bridge

import (
	"errors"
	"fmt"

	"github.com/moby/moby/v2/errdefs"
)

// errInvalidGateway is returned when the user provided default gateway (v4/v6) is not valid.
var errInvalidGateway = errdefs.InvalidParameter(errors.New("default gateway ip must be part of the network"))

// invalidNetworkIDError is returned when the passed
// network id for an existing network is not a known id.
type invalidNetworkIDError string

func (e invalidNetworkIDError) Error() string {
	return fmt.Sprintf("invalid network id %s", string(e))
}

// NotFound denotes the type of this error
func (e invalidNetworkIDError) NotFound() {}

// invalidEndpointIDError is returned when the passed
// endpoint id is not valid.
type invalidEndpointIDError string

func (e invalidEndpointIDError) Error() string {
	return fmt.Sprintf("invalid endpoint id: %s", string(e))
}

// InvalidParameter denotes the type of this error
func (e invalidEndpointIDError) InvalidParameter() {}

// endpointNotFoundError is returned when the no endpoint
// with the passed endpoint id is found.
type endpointNotFoundError string

func (e endpointNotFoundError) Error() string {
	return fmt.Sprintf("endpoint not found: %s", string(e))
}

// NotFound denotes the type of this error
func (e endpointNotFoundError) NotFound() {}
