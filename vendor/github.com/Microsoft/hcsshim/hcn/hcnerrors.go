// Package hcn is a shim for the Host Compute Networking (HCN) service, which manages networking for Windows Server
// containers and Hyper-V containers. Previous to RS5, HCN was referred to as Host Networking Service (HNS).
package hcn

import (
	"errors"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/Microsoft/hcsshim/internal/interop"
	"github.com/sirupsen/logrus"
)

var (
	errInvalidNetworkID      = errors.New("invalid network ID")
	errInvalidEndpointID     = errors.New("invalid endpoint ID")
	errInvalidNamespaceID    = errors.New("invalid namespace ID")
	errInvalidLoadBalancerID = errors.New("invalid load balancer ID")
	errInvalidRouteID        = errors.New("invalid route ID")
)

func checkForErrors(methodName string, hr error, resultBuffer *uint16) error {
	errorFound := false

	if hr != nil {
		errorFound = true
	}

	result := ""
	if resultBuffer != nil {
		result = interop.ConvertAndFreeCoTaskMemString(resultBuffer)
		if result != "" {
			errorFound = true
		}
	}

	if errorFound {
		returnError := new(hr, methodName, result)
		logrus.Debugf(returnError.Error()) // HCN errors logged for debugging.
		return returnError
	}

	return nil
}

type ErrorCode uint32

// For common errors, define the error as it is in windows, so we can quickly determine it later
const (
	ERROR_NOT_FOUND                     = 0x490
	HCN_E_PORT_ALREADY_EXISTS ErrorCode = 0x803b0013
)

type HcnError struct {
	*hcserror.HcsError
	code ErrorCode
}

func (e *HcnError) Error() string {
	return e.HcsError.Error()
}

func CheckErrorWithCode(err error, code ErrorCode) bool {
	hcnError, ok := err.(*HcnError)
	if ok {
		return hcnError.code == code
	}
	return false
}

func IsElementNotFoundError(err error) bool {
	return CheckErrorWithCode(err, ERROR_NOT_FOUND)
}

func IsPortAlreadyExistsError(err error) bool {
	return CheckErrorWithCode(err, HCN_E_PORT_ALREADY_EXISTS)
}

func new(hr error, title string, rest string) error {
	err := &HcnError{}
	hcsError := hcserror.New(hr, title, rest)
	err.HcsError = hcsError.(*hcserror.HcsError)
	err.code = ErrorCode(hcserror.Win32FromError(hr))
	return err
}

//
// Note that the below errors are not errors returned by hcn itself
// we wish to seperate them as they are shim usage error
//

// NetworkNotFoundError results from a failed seach for a network by Id or Name
type NetworkNotFoundError struct {
	NetworkName string
	NetworkID   string
}

func (e NetworkNotFoundError) Error() string {
	if e.NetworkName != "" {
		return fmt.Sprintf("Network name %q not found", e.NetworkName)
	}
	return fmt.Sprintf("Network ID %q not found", e.NetworkID)
}

// EndpointNotFoundError results from a failed seach for an endpoint by Id or Name
type EndpointNotFoundError struct {
	EndpointName string
	EndpointID   string
}

func (e EndpointNotFoundError) Error() string {
	if e.EndpointName != "" {
		return fmt.Sprintf("Endpoint name %q not found", e.EndpointName)
	}
	return fmt.Sprintf("Endpoint ID %q not found", e.EndpointID)
}

// NamespaceNotFoundError results from a failed seach for a namsepace by Id
type NamespaceNotFoundError struct {
	NamespaceID string
}

func (e NamespaceNotFoundError) Error() string {
	return fmt.Sprintf("Namespace ID %q not found", e.NamespaceID)
}

// LoadBalancerNotFoundError results from a failed seach for a loadbalancer by Id
type LoadBalancerNotFoundError struct {
	LoadBalancerId string
}

func (e LoadBalancerNotFoundError) Error() string {
	return fmt.Sprintf("LoadBalancer %q not found", e.LoadBalancerId)
}

// RouteNotFoundError results from a failed seach for a route by Id
type RouteNotFoundError struct {
	RouteId string
}

func (e RouteNotFoundError) Error() string {
	return fmt.Sprintf("SDN Route %q not found", e.RouteId)
}

// IsNotFoundError returns a boolean indicating whether the error was caused by
// a resource not being found.
func IsNotFoundError(err error) bool {
	switch pe := err.(type) {
	case NetworkNotFoundError:
		return true
	case EndpointNotFoundError:
		return true
	case NamespaceNotFoundError:
		return true
	case LoadBalancerNotFoundError:
		return true
	case RouteNotFoundError:
		return true
	case *hcserror.HcsError:
		return pe.Err == hcs.ErrElementNotFound
	}
	return false
}
