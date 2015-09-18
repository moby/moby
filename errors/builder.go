package errors

// This file contains all of the errors that can be generated from the
// docker/builder component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeAtLeastOneArg is generated when the parser comes across a
	// Dockerfile command that doesn't have any args.
	ErrorCodeAtLeastOneArg = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ATLEASTONEARG",
		Message:        "%s requires at least one argument",
		Description:    "The specified command requires at least one argument",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeExactlyOneArg is generated when the parser comes across a
	// Dockerfile command that requires exactly one arg but got less/more.
	ErrorCodeExactlyOneArg = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EXACTLYONEARG",
		Message:        "%s requires exactly one argument",
		Description:    "The specified command requires exactly one argument",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeAtLeastTwoArgs is generated when the parser comes across a
	// Dockerfile command that requires at least two args but got less.
	ErrorCodeAtLeastTwoArgs = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ATLEASTTWOARGS",
		Message:        "%s requires at least two arguments",
		Description:    "The specified command requires at least two arguments",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeTooManyArgs is generated when the parser comes across a
	// Dockerfile command that has more args than it should
	ErrorCodeTooManyArgs = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "TOOMANYARGS",
		Message:        "Bad input to %s, too many args",
		Description:    "The specified command was passed too many arguments",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeChainOnBuild is generated when the parser comes across a
	// Dockerfile command that is trying to chain ONBUILD commands.
	ErrorCodeChainOnBuild = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "CHAINONBUILD",
		Message:        "Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed",
		Description:    "ONBUILD Dockerfile commands aren't allow on ONBUILD commands",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBadOnBuildCmd is generated when the parser comes across a
	// an ONBUILD Dockerfile command with an invalid trigger/command.
	ErrorCodeBadOnBuildCmd = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BADONBUILDCMD",
		Message:        "%s isn't allowed as an ONBUILD trigger",
		Description:    "The specified ONBUILD command isn't allowed",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeMissingFrom is generated when the Dockerfile is missing
	// a FROM command.
	ErrorCodeMissingFrom = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MISSINGFROM",
		Message:        "Please provide a source image with `from` prior to run",
		Description:    "The Dockerfile is missing a FROM command",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNotOnWindows is generated when the specified Dockerfile
	// command is not supported on Windows.
	ErrorCodeNotOnWindows = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOTONWINDOWS",
		Message:        "%s is not supported on Windows",
		Description:    "The specified Dockerfile command is not supported on Windows",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVolumeEmpty is generated when the specified Volume string
	// is empty.
	ErrorCodeVolumeEmpty = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEEMPTY",
		Message:        "Volume specified can not be an empty string",
		Description:    "The specified volume can not be an empty string",
		HTTPStatusCode: http.StatusInternalServerError,
	})
)
