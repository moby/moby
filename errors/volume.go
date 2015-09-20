package errors

// This file contains all of the errors that can be generated from the
// docker/volume component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeLookupVolumeDriver is generated when we look for a volume driver by
	// name and we can't find it.
	ErrorCodeLookupVolumeDriver = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOOKUPVOLUMEDRIVER",
		Message:        "Error looking up volume plugin %s: %v",
		Description:    "The specified volume driver can not be found",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeVolumeExists is generated when create volume checks to see that the volume already exists
	ErrorCodeVolumeExists = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEEXISTS",
		Message:        "volume already exists under %s",
		Description:    "The specified volume already exists",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeRemovingDirectory is generated when removing a volume that does not
	// exist within the scope of the driver.
	ErrorCodeRemovingDirectory = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "REMOVINGDIRECTORY",
		Message:        "Unable to remove a directory of out the Docker root %s: %s",
		Description:    "The specified volume directory can not be found in the scope",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeVolumeInUse is a typed error returned when trying to remove a volume that is currently in use by a container
	ErrorCodeVolumeInUse = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VOLUMEINUSE",
		Message:        "volume is in use",
		Description:    "The specified volume is in use",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeNoSuchVolume is a typed error returned if the requested volume doesn't exist in the volume store
	ErrorCodeNoSuchVolume = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSUCHVOLUME",
		Message:        "no such volume",
		Description:    "The specified volume does not exist",
		HTTPStatusCode: http.StatusNotFound,
	})
)
