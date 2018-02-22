package distribution // import "github.com/docker/docker/distribution"

import (
	"errors"
	"strings"
	"syscall"
	"testing"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/client"
)

var alwaysContinue = []error{
	&client.UnexpectedHTTPResponseError{},

	// Some errcode.Errors that don't disprove the existence of a V1 image
	errcode.Error{Code: errcode.ErrorCodeUnauthorized},
	errcode.Error{Code: v2.ErrorCodeManifestUnknown},
	errcode.Error{Code: v2.ErrorCodeNameUnknown},

	errors.New("some totally unexpected error"),
}

var continueFromMirrorEndpoint = []error{
	ImageConfigPullError{},

	// Some other errcode.Error that doesn't indicate we should search for a V1 image.
	errcode.Error{Code: errcode.ErrorCodeTooManyRequests},
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
	errs := []error{}
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

func TestContinueOnError_UnnestsErrors(t *testing.T) {
	// ContinueOnError should evaluate nested errcode.Errors.

	// Assumes that v2.ErrorCodeNameUnknown is a continueable error code.
	err := errcode.Errors{errcode.Error{Code: v2.ErrorCodeNameUnknown}}
	if !continueOnError(err, false) {
		t.Fatal("ContinueOnError should unnest, base return value on errcode.Errors")
	}

	// Assumes that errcode.ErrorCodeTooManyRequests is not a V1-fallback indication
	err = errcode.Errors{errcode.Error{Code: errcode.ErrorCodeTooManyRequests}}
	if continueOnError(err, false) {
		t.Fatal("ContinueOnError should unnest, base return value on errcode.Errors")
	}
}
