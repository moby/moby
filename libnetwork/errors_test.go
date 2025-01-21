package libnetwork

import (
	"testing"

	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libnetwork/types"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestErrorInterfaces(t *testing.T) {
	badRequestErrorList := []error{ErrInvalidID(""), ErrInvalidName("")}
	for _, err := range badRequestErrorList {
		assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
	}

	maskableErrorList := []error{ManagerRedirectError("")}
	for _, err := range maskableErrorList {
		switch u := err.(type) {
		case types.MaskableError:
		default:
			t.Errorf("Failed to detect err %v is of type MaskableError. Got type: %T", err, u)
		}
	}

	notFoundErrorList := []error{&UnknownNetworkError{}, ErrNoSuchNetwork(""), ErrNoSuchEndpoint("")}
	for _, err := range notFoundErrorList {
		assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	}

	forbiddenErrorList := []error{&ActiveContainerError{}}
	for _, err := range forbiddenErrorList {
		assert.Check(t, is.ErrorType(err, errdefs.IsForbidden))
	}
}
