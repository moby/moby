package errors

// This file contains all of the errors that can be generated from the
// docker/utils component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeUsingGitClone is generated when git throws an error while cloning
	// a repository.
	ErrorCodeUsingGitClone = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "USINGGITCLONE",
		Message:        "Error trying to use git: %s (%s)",
		Description:    "The specified error was thrown while using git to clone a repository",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeUsingGitCheckout is generated when git throws an error during
	// a checkout.
	ErrorCodeUsingGitCheckout = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "USINGGITCHECKOUT",
		Message:        "Error trying to use git: %s (%s)",
		Description:    "The specified error was thrown while using git checkout",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeSettingGitContext is generated when an error occurs while
	// setting context.
	ErrorCodeSettingGitContext = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "SETTINGGITCONTEXT",
		Message:        "Error setting git context, %q not within git root: %s",
		Description:    "The specified error was thrown while setting the specified context",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGitContextMustBeSetToADirectory is generated when the context
	// being set for git is not to a directory.
	ErrorCodeGitContextMustBeSetToADirectory = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GITCONTEXTMUSTBESETTOADIRECTORY",
		Message:        "Error setting git context, not a directory: %s",
		Description:    "The specified context is not a directory",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodePermissionWalkingFileTree is generated when a permission
	// error is thrown attempting to walk the specified path.
	ErrorCodePermissionWalkingFileTree = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PERMISSIONWALKINGFILETREE",
		Message:        "can't stat '%s'",
		Description:    "No permission to access the specified path",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodePermissionOpeningFile is generated when a permission
	// error is thrown attempting to open the specified file.
	ErrorCodePermissionOpeningFile = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PERMISSIONOPENINGFILE",
		Message:        "no permission to read from '%s'",
		Description:    "No permission to open the specified file",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeOpeningDockerIgnore is generated when an error is thrown
	// attempting to open .dockerignore file.
	ErrorCodeOpeningDockerIgnore = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "OPENINGDOCKERIGNORE",
		Message:        "Error reading '%s': %v",
		Description:    "The specified error was thrown opening the specified .dockerignore file",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeScanningDockerIgnore is generated when an error is thrown
	// attempting to scan the .dockerignore file.
	ErrorCodeScanningDockerIgnore = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "SCANNINGDOCKERIGNORE",
		Message:        "Error reading '%s': %v",
		Description:    "The specified error was thrown scanning the specified .dockerignore file",
		HTTPStatusCode: http.StatusInternalServerError,
	})
)
