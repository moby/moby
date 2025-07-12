package distribution

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"syscall"

	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/docker/distribution/registry/api/errcode"
	v2 "github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

// fallbackError wraps an error that can possibly allow fallback to a different
// endpoint.
type fallbackError struct {
	// err is the error being wrapped.
	err error
	// transportOK is set to true if we managed to speak HTTP with the
	// registry. This confirms that we're using appropriate TLS settings
	// (or lack of TLS).
	transportOK bool
}

// Error renders the FallbackError as a string.
func (f fallbackError) Error() string {
	return f.err.Error()
}

func (f fallbackError) Cause() error {
	return f.err
}

func (f fallbackError) Unwrap() error {
	return f.err
}

type notFoundError struct {
	cause errcode.Error
	ref   reference.Named
}

func (e notFoundError) Error() string {
	switch e.cause.Code {
	case errcode.ErrorCodeDenied:
		// ErrorCodeDenied is used when access to the repository was denied
		return errors.Wrapf(e.cause, "pull access denied for %s, repository does not exist or may require 'docker login'", reference.FamiliarName(e.ref)).Error()
	case v2.ErrorCodeManifestUnknown:
		return errors.Wrapf(e.cause, "manifest for %s not found", reference.FamiliarString(e.ref)).Error()
	case v2.ErrorCodeNameUnknown:
		return errors.Wrapf(e.cause, "repository %s not found", reference.FamiliarName(e.ref)).Error()
	}
	// Shouldn't get here, but this is better than returning an empty string
	return e.cause.Message
}

func (e notFoundError) NotFound() {}

func (e notFoundError) Cause() error {
	return e.cause
}

func (e notFoundError) Unwrap() error {
	return e.cause
}

// unsupportedMediaTypeError is an error issued when attempted
// to pull unsupported content.
type unsupportedMediaTypeError struct {
	MediaType string
}

func (e unsupportedMediaTypeError) InvalidParameter() {}

// Error returns the error string for unsupportedMediaTypeError.
func (e unsupportedMediaTypeError) Error() string {
	return "unsupported media type " + e.MediaType
}

// translatePullError is used to convert an error from a registry pull
// operation to an error representing the entire pull operation. Any error
// information which is not used by the returned error gets output to
// log at info level.
func translatePullError(err error, ref reference.Named) error {
	// FIXME(thaJeztah): cleanup error and context handling in this package, as it's really messy.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var ecsErr errcode.Errors
	var ecErr errcode.Error
	var dnrErr xfer.DoNotRetry
	switch {
	case errors.As(err, &ecsErr):
		if len(ecsErr) != 0 {
			for _, extra := range ecsErr[1:] {
				log.G(context.TODO()).WithError(extra).Infof("Ignoring extra error returned from registry")
			}
			return translatePullError(ecsErr[0], ref)
		}
	case errors.As(err, &ecErr):
		switch ecErr.Code {
		case errcode.ErrorCodeDenied, v2.ErrorCodeManifestUnknown, v2.ErrorCodeNameUnknown:
			return notFoundError{ecErr, ref}
		}
	case errors.As(err, &dnrErr):
		return translatePullError(dnrErr.Err, ref)
	}

	return errdefs.Unknown(err)
}

// continueOnError returns true if we should fallback to the next endpoint
// as a result of this error.
func continueOnError(err error, mirrorEndpoint bool) bool {
	// FIXME(thaJeztah): cleanup error and context handling in this package, as it's really messy.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var ecsErr errcode.Errors
	var ecErr errcode.Error
	var unexpectedHTTPResponseErr *client.UnexpectedHTTPResponseError
	var imageConfigPullErr imageConfigPullError
	var unsupportedMediaTypeErr unsupportedMediaTypeError
	var e error
	switch {
	case errors.As(err, &ecsErr):
		if len(ecsErr) == 0 {
			return true
		}
		return continueOnError(ecsErr[0], mirrorEndpoint)
	case errors.As(err, &ecErr):
		return mirrorEndpoint
	case errors.As(err, &unexpectedHTTPResponseErr):
		return true
	case errors.As(err, &imageConfigPullErr):
		// imageConfigPullError only happens with v2 images, v1 fallback is
		// unnecessary.
		// Failures from a mirror endpoint should result in fallback to the
		// canonical repo.
		return mirrorEndpoint
	case errors.As(err, unsupportedMediaTypeErr):
		return false
	case errors.As(err, &e):
		return !strings.Contains(err.Error(), strings.ToLower(syscall.ESRCH.Error()))
	default:
		// let's be nice and fallback if the error is a completely
		// unexpected one.
		// If new errors have to be handled in some way, please
		// add them to the switch above.
		return true
	}
}

// retryOnError wraps the error in xfer.DoNotRetry if we should not retry the
// operation after this error.
func retryOnError(err error) error {
	var ecsErr errcode.Errors
	var ecErr errcode.Error
	var urlErr *url.Error
	var unexpectedHTTPResponseErr *client.UnexpectedHTTPResponseError
	var unsupportedMediaTypeErr unsupportedMediaTypeError
	var e error
	switch {
	case errors.As(err, &ecsErr):
		if len(ecsErr) != 0 {
			return retryOnError(ecsErr[0])
		}
	case errors.As(err, &ecErr):
		switch ecErr.Code {
		case errcode.ErrorCodeUnauthorized, errcode.ErrorCodeUnsupported, errcode.ErrorCodeDenied, errcode.ErrorCodeTooManyRequests, v2.ErrorCodeNameUnknown:
			return xfer.DoNotRetry{Err: err}
		}
	case errors.As(err, &urlErr):
		switch {
		case errors.Is(urlErr.Err, auth.ErrNoBasicAuthCredentials), errors.Is(urlErr.Err, auth.ErrNoToken):
			return xfer.DoNotRetry{Err: urlErr.Err}
		}
		return retryOnError(urlErr.Err)
	case errors.As(err, &unexpectedHTTPResponseErr), errors.As(err, &unsupportedMediaTypeErr):
		return xfer.DoNotRetry{Err: err}
	case errors.As(err, &e):
		if errors.Is(err, distribution.ErrBlobUnknown) {
			return xfer.DoNotRetry{Err: err}
		}
		if strings.Contains(err.Error(), strings.ToLower(syscall.ENOSPC.Error())) {
			return xfer.DoNotRetry{Err: err}
		}
	}
	// let's be nice and fallback if the error is a completely
	// unexpected one.
	// If new errors have to be handled in some way, please
	// add them to the switch above.
	return err
}

type AIModelNotSupportedError struct{}

func (e AIModelNotSupportedError) Error() string {
	return `AI models are not yet supported by the Engine, please use "docker model pull/run" instead`
}

func (e AIModelNotSupportedError) InvalidParameter() {}

type invalidManifestClassError struct {
	mediaType string
	class     string
}

func (e invalidManifestClassError) Error() string {
	return fmt.Sprintf("Encountered remote %q(%s) when fetching", e.mediaType, e.class)
}

func (e invalidManifestClassError) InvalidParameter() {}

type invalidManifestFormatError struct{}

func (invalidManifestFormatError) Error() string {
	return "unsupported manifest format"
}

func (invalidManifestFormatError) InvalidParameter() {}

type reservedNameError string

func (e reservedNameError) Error() string {
	return "'" + string(e) + "' is a reserved name"
}

func (e reservedNameError) Forbidden() {}

type invalidArgumentErr struct{ error }

func (invalidArgumentErr) InvalidParameter() {}

func DeprecatedSchema1ImageError(ref reference.Named) error {
	msg := "Docker Image Format v1 and Docker Image manifest version 2, schema 1 support has been removed."
	if ref != nil {
		msg += " Suggest the author of " + ref.String() + " to upgrade the image to the OCI Format or Docker Image manifest v2, schema 2."
	}
	msg += " More information at https://docs.docker.com/go/deprecated-image-specs/"
	return invalidArgumentErr{errors.New(msg)}
}
