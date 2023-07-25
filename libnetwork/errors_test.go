package libnetwork

import (
	"testing"

	"github.com/docker/docker/libnetwork/types"
)

func TestErrorInterfaces(t *testing.T) {
	badRequestErrorList := []error{ErrInvalidID(""), ErrInvalidName(""), ErrInvalidJoin{}, ErrInvalidNetworkDriver(""), InvalidContainerIDError("")}
	for _, err := range badRequestErrorList {
		switch u := err.(type) {
		case types.BadRequestError:
		default:
			t.Errorf("Failed to detect err %v is of type BadRequestError. Got type: %T", err, u)
		}
	}

	maskableErrorList := []error{ErrNoContainer{}}
	for _, err := range maskableErrorList {
		switch u := err.(type) {
		case types.MaskableError:
		default:
			t.Errorf("Failed to detect err %v is of type MaskableError. Got type: %T", err, u)
		}
	}

	notFoundErrorList := []error{NetworkTypeError(""), &UnknownNetworkError{}, &UnknownEndpointError{}, ErrNoSuchNetwork(""), ErrNoSuchEndpoint("")}
	for _, err := range notFoundErrorList {
		switch u := err.(type) {
		case types.NotFoundError:
		default:
			t.Errorf("Failed to detect err %v is of type NotFoundError. Got type: %T", err, u)
		}
	}

	forbiddenErrorList := []error{&ActiveContainerError{}}
	for _, err := range forbiddenErrorList {
		switch u := err.(type) {
		case types.ForbiddenError:
		default:
			t.Errorf("Failed to detect err %v is of type ForbiddenError. Got type: %T", err, u)
		}
	}
}
