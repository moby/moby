package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errhttp"
	"github.com/docker/docker/api/types/versions"
)

// errConnectionFailed implements an error returned when connection failed.
type errConnectionFailed struct {
	error
}

// Error returns a string representation of an errConnectionFailed
func (e errConnectionFailed) Error() string {
	return e.error.Error()
}

func (e errConnectionFailed) Unwrap() error {
	return e.error
}

// IsErrConnectionFailed returns true if the error is caused by connection failed.
func IsErrConnectionFailed(err error) bool {
	return errors.As(err, &errConnectionFailed{})
}

// ErrorConnectionFailed returns an error with host in the error message when connection to docker daemon failed.
//
// Deprecated: this function was only used internally, and will be removed in the next release.
func ErrorConnectionFailed(host string) error {
	return connectionFailed(host)
}

// connectionFailed returns an error with host in the error message when connection
// to docker daemon failed.
func connectionFailed(host string) error {
	var err error
	if host == "" {
		err = errors.New("Cannot connect to the Docker daemon. Is the docker daemon running on this host?")
	} else {
		err = fmt.Errorf("Cannot connect to the Docker daemon at %s. Is the docker daemon running?", host)
	}
	return errConnectionFailed{error: err}
}

// IsErrNotFound returns true if the error is a NotFound error, which is returned
// by the API when some object is not found. It is an alias for [cerrdefs.IsNotFound].
//
// Deprecated: use [cerrdefs.IsNotFound] instead.
func IsErrNotFound(err error) bool {
	return cerrdefs.IsNotFound(err)
}

type objectNotFoundError struct {
	object string
	id     string
}

func (e objectNotFoundError) NotFound() {}

func (e objectNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such %s: %s", e.object, e.id)
}

// NewVersionError returns an error if the APIVersion required is less than the
// current supported version.
//
// It performs API-version negotiation if the Client is configured with this
// option, otherwise it assumes the latest API version is used.
func (cli *Client) NewVersionError(ctx context.Context, APIrequired, feature string) error {
	// Make sure we negotiated (if the client is configured to do so),
	// as code below contains API-version specific handling of options.
	//
	// Normally, version-negotiation (if enabled) would not happen until
	// the API request is made.
	if err := cli.checkVersion(ctx); err != nil {
		return err
	}
	if cli.version != "" && versions.LessThan(cli.version, APIrequired) {
		return fmt.Errorf("%q requires API version %s, but the Docker daemon API version is %s", feature, APIrequired, cli.version)
	}
	return nil
}

type httpError struct {
	err    error
	errdef error
}

func (e *httpError) Error() string {
	return e.err.Error()
}

func (e *httpError) Unwrap() error {
	return e.err
}

func (e *httpError) Is(target error) bool {
	return errors.Is(e.errdef, target)
}

// httpErrorFromStatusCode creates an errdef error, based on the provided HTTP status-code
func httpErrorFromStatusCode(err error, statusCode int) error {
	if err == nil {
		return nil
	}
	base := errhttp.ToNative(statusCode)
	if base != nil {
		return &httpError{err: err, errdef: base}
	}

	switch {
	case statusCode >= http.StatusOK && statusCode < http.StatusBadRequest:
		// it's a client error
		return err
	case statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError:
		return &httpError{err: err, errdef: cerrdefs.ErrInvalidArgument}
	case statusCode >= http.StatusInternalServerError && statusCode < 600:
		return &httpError{err: err, errdef: cerrdefs.ErrInternal}
	default:
		return &httpError{err: err, errdef: cerrdefs.ErrUnknown}
	}
}
