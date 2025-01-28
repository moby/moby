//go:build linux

package bridge

import (
	"errors"
	"fmt"

	"github.com/docker/docker/errdefs"
)

// errInvalidGateway is returned when the user provided default gateway (v4/v6) is not valid.
var errInvalidGateway = errdefs.InvalidParameter(errors.New("default gateway ip must be part of the network"))

// InvalidNetworkIDError is returned when the passed
// network id for an existing network is not a known id.
type InvalidNetworkIDError string

func (inie InvalidNetworkIDError) Error() string {
	return fmt.Sprintf("invalid network id %s", string(inie))
}

// NotFound denotes the type of this error
func (inie InvalidNetworkIDError) NotFound() {}

// InvalidEndpointIDError is returned when the passed
// endpoint id is not valid.
type InvalidEndpointIDError string

func (ieie InvalidEndpointIDError) Error() string {
	return fmt.Sprintf("invalid endpoint id: %s", string(ieie))
}

// InvalidParameter denotes the type of this error
func (ieie InvalidEndpointIDError) InvalidParameter() {}

// EndpointNotFoundError is returned when the no endpoint
// with the passed endpoint id is found.
type EndpointNotFoundError string

func (enfe EndpointNotFoundError) Error() string {
	return fmt.Sprintf("endpoint not found: %s", string(enfe))
}

// NotFound denotes the type of this error
func (enfe EndpointNotFoundError) NotFound() {}
