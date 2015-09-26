package errors

// This file contains all of the errors that can be generated from the
// docker/docker component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeFailCreateConfDir is generated when docker fails to create the configuration directory during migration.
	ErrorCodeFailCreateConfDir = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "FAILCREATECONFDIR",
		Message:        "Unable to create daemon configuration directory: %s",
		Description:    "Docker fails to create the configuration directory during migration",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeFailCreateKeyFile is generated when docker fails to create the key file  during migration.
	ErrorCodeFailCreateKeyFile = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "FAILCREATEKEYFILE",
		Message:        "error creating key file %q: %s",
		Description:    "Docker fails to create the key file during migration",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeFailOpenKeyFile is generated when docker fails to open the key file  during migration.
	ErrorCodeFailOpenKeyFile = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "FAILOPENKEYFILE",
		Message:        "error opening key file %q: %s",
		Description:    "Docker fails to create the key file during migration",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeFailCopyKeyFile is generated when docker fails to copy the key file  during migration.
	ErrorCodeFailCopyKeyFile = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "FAILCOPYKEYFILE",
		Message:        "error copying key: %s",
		Description:    "Docker fails to copy the key file during migration",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeSetUmaskFailed is generated when docker fails to set umask.
	ErrorCodeSetUmaskFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "SETUMASKFAILED",
		Message:        "failed to set umask: expected %#o, got %#o",
		Description:    "Docker fails to set umask to configuration files",
		HTTPStatusCode: http.StatusInternalServerError,
	})
)
