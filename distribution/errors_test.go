package distribution // import "github.com/docker/docker/distribution"

import (
	"errors"
	"strings"
	"syscall"
	"testing"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/client"
)

var errUnexpected = errors.New("some totally unexpected error")

var alwaysContinue = []error{
	&client.UnexpectedHTTPResponseError{},
	errcode.Errors{},
	errUnexpected,
	// nested
	errcode.Errors{errUnexpected},
}

var continueFromMirrorEndpoint = []error{
	imageConfigPullError{},
	errcode.Error{},
	// nested
	errcode.Errors{errcode.Error{}},
}

var neverContinue = []error{
	errors.New(strings.ToLower(syscall.ESRCH.Error())), // No such process
}

func TestContinueOnError_NonMirrorEndpoint(t *testing.T) {
	for _, err := range alwaysContinue {
		if !continueOnError(err, false) {
			t.Errorf("Should continue from non-mirror endpoint: %T: '%s'", err, err.Error())
		}
	}

	for _, err := range continueFromMirrorEndpoint {
		if continueOnError(err, false) {
			t.Errorf("Should only continue from mirror endpoint: %T: '%s'", err, err.Error())
		}
	}
}

func TestContinueOnError_MirrorEndpoint(t *testing.T) {
	var errs []error
	errs = append(errs, alwaysContinue...)
	errs = append(errs, continueFromMirrorEndpoint...)
	for _, err := range errs {
		if !continueOnError(err, true) {
			t.Errorf("Should continue from mirror endpoint: %T: '%s'", err, err.Error())
		}
	}
}

func TestContinueOnError_NeverContinue(t *testing.T) {
	for _, isMirrorEndpoint := range []bool{true, false} {
		for _, err := range neverContinue {
			if continueOnError(err, isMirrorEndpoint) {
				t.Errorf("Should never continue: %T: '%s'", err, err.Error())
			}
		}
	}
}
