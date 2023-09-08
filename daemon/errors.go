package daemon // import "github.com/docker/docker/daemon"

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"google.golang.org/grpc/status"
)

func isNotRunning(err error) bool {
	var nre *containerNotRunningError
	return errors.As(err, &nre)
}

func errNotRunning(id string) error {
	return &containerNotRunningError{errors.Errorf("container %s is not running", id)}
}

type containerNotRunningError struct {
	error
}

func (e containerNotRunningError) Conflict() {}

func containerNotFound(id string) error {
	return objNotFoundError{"container", id}
}

type objNotFoundError struct {
	object string
	id     string
}

func (e objNotFoundError) Error() string {
	return "No such " + e.object + ": " + e.id
}

func (e objNotFoundError) NotFound() {}

func errContainerIsRestarting(containerID string) error {
	cause := errors.Errorf("Container %s is restarting, wait until the container is running", containerID)
	return errdefs.Conflict(cause)
}

func errExecNotFound(id string) error {
	return objNotFoundError{"exec instance", id}
}

func errExecPaused(id string) error {
	cause := errors.Errorf("Container %s is paused, unpause the container before exec", id)
	return errdefs.Conflict(cause)
}

func errNotPaused(id string) error {
	cause := errors.Errorf("Container %s is already paused", id)
	return errdefs.Conflict(cause)
}

type nameConflictError struct {
	id   string
	name string
}

func (e nameConflictError) Error() string {
	return fmt.Sprintf("Conflict. The container name %q is already in use by container %q. You have to remove (or rename) that container to be able to reuse that name.", e.name, e.id)
}

func (nameConflictError) Conflict() {}

type invalidIdentifier string

func (e invalidIdentifier) Error() string {
	return fmt.Sprintf("invalid name or ID supplied: %q", string(e))
}

func (invalidIdentifier) InvalidParameter() {}

type incompatibleDeviceRequest struct {
	driver string
	caps   [][]string
}

func (i incompatibleDeviceRequest) Error() string {
	return fmt.Sprintf("could not select device driver %q with capabilities: %v", i.driver, i.caps)
}

func (incompatibleDeviceRequest) InvalidParameter() {}

type duplicateMountPointError string

func (e duplicateMountPointError) Error() string {
	return "Duplicate mount point: " + string(e)
}
func (duplicateMountPointError) InvalidParameter() {}

type containerFileNotFound struct {
	file      string
	container string
}

func (e containerFileNotFound) Error() string {
	return "Could not find the file " + e.file + " in container " + e.container
}

func (containerFileNotFound) NotFound() {}

type startInvalidConfigError string

func (e startInvalidConfigError) Error() string {
	return string(e)
}

func (e startInvalidConfigError) InvalidParameter() {} // Is this right???

// exitStatus is the exit-code as set by setExitCodeFromError
type exitStatus = int

const (
	exitEaccess     exitStatus = 126 // container cmd can't be invoked (permission denied)
	exitCmdNotFound exitStatus = 127 // container cmd not found/does not exist or invalid bind-mount
	exitUnknown     exitStatus = 128 // unknown error
)

// setExitCodeFromError converts the error returned by containerd
// when starting a container, and applies the corresponding exitStatus to the
// container. It returns an errdefs error (either errdefs.ErrInvalidParameter
// or errdefs.ErrUnknown).
func setExitCodeFromError(setExitCode func(exitStatus), err error) error {
	if err == nil {
		return nil
	}
	errDesc := status.Convert(err).Message()
	contains := func(s1, s2 string) bool {
		return strings.Contains(strings.ToLower(s1), s2)
	}

	// set to 126 for container cmd can't be invoked errors
	if contains(errDesc, syscall.EACCES.Error()) {
		setExitCode(exitEaccess)
		return startInvalidConfigError(errDesc)
	}

	// Go 1.20 changed the error for attempting to execute a directory from
	// syscall.EACCESS to syscall.EISDIR. Unfortunately docker/cli checks
	// whether the error message contains syscall.EACCESS.Error() to
	// determine whether to exit with code 126 or 125, so we have little
	// choice but to fudge the error string.
	if contains(errDesc, syscall.EISDIR.Error()) {
		errDesc += ": " + syscall.EACCES.Error()
		setExitCode(exitEaccess)
		return startInvalidConfigError(errDesc)
	}

	// attempted to mount a file onto a directory, or a directory onto a file, maybe from user specified bind mounts
	if contains(errDesc, syscall.ENOTDIR.Error()) {
		errDesc += ": Are you trying to mount a directory onto a file (or vice-versa)? Check if the specified host path exists and is the expected type"
		setExitCode(exitCmdNotFound)
		return startInvalidConfigError(errDesc)
	}

	// if we receive an internal error from the initial start of a container then lets
	// return it instead of entering the restart loop
	// set to 127 for container cmd not found/does not exist.
	if isInvalidCommand(errDesc) {
		setExitCode(exitCmdNotFound)
		return startInvalidConfigError(errDesc)
	}

	// TODO: it would be nice to get some better errors from containerd so we can return better errors here
	setExitCode(exitUnknown)
	return errdefs.Unknown(errors.New(errDesc))
}

// isInvalidCommand tries to detect if the reason the container failed to start
// was due to an invalid command for the container (command not found, or not
// a valid executable).
func isInvalidCommand(errMessage string) bool {
	errMessage = strings.ToLower(errMessage)
	errMessages := []string{
		"executable file not found",
		"no such file or directory",
		"system cannot find the file specified",
		"failed to run runc create/exec call",
	}

	for _, msg := range errMessages {
		if strings.Contains(errMessage, msg) {
			return true
		}
	}
	return false
}
