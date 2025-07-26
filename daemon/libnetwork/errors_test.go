package libnetwork

import (
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestErrorInterfaces(t *testing.T) {
	maskableErrorList := []error{ManagerRedirectError("")}
	for _, err := range maskableErrorList {
		switch u := err.(type) {
		case types.MaskableError:
		default:
			t.Errorf("Failed to detect err %v is of type MaskableError. Got type: %T", err, u)
		}
	}

	notFoundErrorList := []error{ErrNoSuchNetwork("")}
	for _, err := range notFoundErrorList {
		assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	}

	forbiddenErrorList := []error{&ActiveContainerError{}}
	for _, err := range forbiddenErrorList {
		assert.Check(t, is.ErrorType(err, cerrdefs.IsPermissionDenied))
	}
}
