package client

import (
	"fmt"

	"net/http"

	"github.com/docker/docker/api/types/versions"
	"github.com/pkg/errors"
)

// errConnectionFailed implements an error returned when connection failed.
type errConnectionFailed struct {
	host string
}

// Error returns a string representation of an errConnectionFailed
func (err errConnectionFailed) Error() string {
	if err.host == "" {
		return "Cannot connect to the Docker daemon. Is the docker daemon running on this host?"
	}
	return fmt.Sprintf("Cannot connect to the Docker daemon at %s. Is the docker daemon running?", err.host)
}

// IsErrConnectionFailed returns true if the error is caused by connection failed.
func IsErrConnectionFailed(err error) bool {
	_, ok := errors.Cause(err).(errConnectionFailed)
	return ok
}

// ErrorConnectionFailed returns an error with host in the error message when connection to docker daemon failed.
func ErrorConnectionFailed(host string) error {
	return errConnectionFailed{host: host}
}

type notFound interface {
	error
	NotFound() bool // Is the error a NotFound error
}

// IsErrNotFound returns true if the error is a NotFound error, which is returned
// by the API when some object is not found.
func IsErrNotFound(err error) bool {
	te, ok := err.(notFound)
	return ok && te.NotFound()
}

type objectNotFoundError struct {
	object string
	id     string
}

func (e objectNotFoundError) NotFound() bool {
	return true
}

func (e objectNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such %s: %s", e.object, e.id)
}

func wrapResponseError(err error, resp serverResponse, object, id string) error {
	switch {
	case err == nil:
		return nil
	case resp.statusCode == http.StatusNotFound:
		return objectNotFoundError{object: object, id: id}
	default:
		return err
	}
}

// IsErrImageNotFound returns true if the error is caused
// when an image is not found in the docker host.
//
// Deprecated: Use IsErrNotFound
func IsErrImageNotFound(err error) bool {
	return IsErrNotFound(err)
}

// IsErrContainerNotFound returns true if the error is caused
// when a container is not found in the docker host.
//
// Deprecated: Use IsErrNotFound
func IsErrContainerNotFound(err error) bool {
	return IsErrNotFound(err)
}

// IsErrNetworkNotFound returns true if the error is caused
// when a network is not found in the docker host.
//
// Deprecated: Use IsErrNotFound
func IsErrNetworkNotFound(err error) bool {
	return IsErrNotFound(err)
}

// IsErrVolumeNotFound returns true if the error is caused
// when a volume is not found in the docker host.
//
// Deprecated: Use IsErrNotFound
func IsErrVolumeNotFound(err error) bool {
	return IsErrNotFound(err)
}

// unauthorizedError represents an authorization error in a remote registry.
type unauthorizedError struct {
	cause error
}

// Error returns a string representation of an unauthorizedError
func (u unauthorizedError) Error() string {
	return u.cause.Error()
}

// IsErrUnauthorized returns true if the error is caused
// when a remote registry authentication fails
func IsErrUnauthorized(err error) bool {
	_, ok := err.(unauthorizedError)
	return ok
}

// IsErrNodeNotFound returns true if the error is caused
// when a node is not found.
//
// Deprecated: Use IsErrNotFound
func IsErrNodeNotFound(err error) bool {
	return IsErrNotFound(err)
}

// IsErrServiceNotFound returns true if the error is caused
// when a service is not found.
//
// Deprecated: Use IsErrNotFound
func IsErrServiceNotFound(err error) bool {
	return IsErrNotFound(err)
}

// IsErrTaskNotFound returns true if the error is caused
// when a task is not found.
//
// Deprecated: Use IsErrNotFound
func IsErrTaskNotFound(err error) bool {
	return IsErrNotFound(err)
}

type pluginPermissionDenied struct {
	name string
}

func (e pluginPermissionDenied) Error() string {
	return "Permission denied while installing plugin " + e.name
}

// IsErrPluginPermissionDenied returns true if the error is caused
// when a user denies a plugin's permissions
func IsErrPluginPermissionDenied(err error) bool {
	_, ok := err.(pluginPermissionDenied)
	return ok
}

// NewVersionError returns an error if the APIVersion required
// if less than the current supported version
func (cli *Client) NewVersionError(APIrequired, feature string) error {
	if cli.version != "" && versions.LessThan(cli.version, APIrequired) {
		return fmt.Errorf("%q requires API version %s, but the Docker daemon API version is %s", feature, APIrequired, cli.version)
	}
	return nil
}

// IsErrSecretNotFound returns true if the error is caused
// when a secret is not found.
//
// Deprecated: Use IsErrNotFound
func IsErrSecretNotFound(err error) bool {
	return IsErrNotFound(err)
}

// IsErrConfigNotFound returns true if the error is caused
// when a config is not found.
//
// Deprecated: Use IsErrNotFound
func IsErrConfigNotFound(err error) bool {
	return IsErrNotFound(err)
}

// IsErrPluginNotFound returns true if the error is caused
// when a plugin is not found in the docker host.
//
// Deprecated: Use IsErrNotFound
func IsErrPluginNotFound(err error) bool {
	return IsErrNotFound(err)
}
