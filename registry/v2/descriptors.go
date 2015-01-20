package v2

import "net/http"

// TODO(stevvooe): Add route descriptors for each named route, along with
// accepted methods, parameters, returned status codes and error codes.

// ErrorDescriptor provides relevant information about a given error code.
type ErrorDescriptor struct {
	// Code is the error code that this descriptor describes.
	Code ErrorCode

	// Value provides a unique, string key, often captilized with
	// underscores, to identify the error code. This value is used as the
	// keyed value when serializing api errors.
	Value string

	// Message is a short, human readable decription of the error condition
	// included in API responses.
	Message string

	// Description provides a complete account of the errors purpose, suitable
	// for use in documentation.
	Description string

	// HTTPStatusCodes provides a list of status under which this error
	// condition may arise. If it is empty, the error condition may be seen
	// for any status code.
	HTTPStatusCodes []int
}

// ErrorDescriptors provides a list of HTTP API Error codes that may be
// encountered when interacting with the registry API.
var ErrorDescriptors = []ErrorDescriptor{
	{
		Code:    ErrorCodeUnknown,
		Value:   "UNKNOWN",
		Message: "unknown error",
		Description: `Generic error returned when the error does not have an
		API classification.`,
	},
	{
		Code:    ErrorCodeDigestInvalid,
		Value:   "DIGEST_INVALID",
		Message: "provided digest did not match uploaded content",
		Description: `When a blob is uploaded, the registry will check that
		the content matches the digest provided by the client. The error may
		include a detail structure with the key "digest", including the
		invalid digest string. This error may also be returned when a manifest
		includes an invalid layer digest.`,
		HTTPStatusCodes: []int{http.StatusBadRequest, http.StatusNotFound},
	},
	{
		Code:    ErrorCodeSizeInvalid,
		Value:   "SIZE_INVALID",
		Message: "provided length did not match content length",
		Description: `When a layer is uploaded, the provided size will be
		checked against the uploaded content. If they do not match, this error
		will be returned.`,
		HTTPStatusCodes: []int{http.StatusBadRequest},
	},
	{
		Code:    ErrorCodeNameInvalid,
		Value:   "NAME_INVALID",
		Message: "manifest name did not match URI",
		Description: `During a manifest upload, if the name in the manifest
		does not match the uri name, this error will be returned.`,
		HTTPStatusCodes: []int{http.StatusBadRequest, http.StatusNotFound},
	},
	{
		Code:    ErrorCodeTagInvalid,
		Value:   "TAG_INVALID",
		Message: "manifest tag did not match URI",
		Description: `During a manifest upload, if the tag in the manifest
		does not match the uri tag, this error will be returned.`,
		HTTPStatusCodes: []int{http.StatusBadRequest, http.StatusNotFound},
	},
	{
		Code:    ErrorCodeNameUnknown,
		Value:   "NAME_UNKNOWN",
		Message: "repository name not known to registry",
		Description: `This is returned if the name used during an operation is
		unknown to the registry.`,
		HTTPStatusCodes: []int{http.StatusNotFound},
	},
	{
		Code:    ErrorCodeManifestUnknown,
		Value:   "MANIFEST_UNKNOWN",
		Message: "manifest unknown",
		Description: `This error is returned when the manifest, identified by
		name and tag is unknown to the repository.`,
		HTTPStatusCodes: []int{http.StatusNotFound},
	},
	{
		Code:    ErrorCodeManifestInvalid,
		Value:   "MANIFEST_INVALID",
		Message: "manifest invalid",
		Description: `During upload, manifests undergo several checks ensuring
		validity. If those checks fail, this error may be returned, unless a
		more specific error is included. The detail will contain information
		the failed validation.`,
		HTTPStatusCodes: []int{http.StatusBadRequest},
	},
	{
		Code:    ErrorCodeManifestUnverified,
		Value:   "MANIFEST_UNVERIFIED",
		Message: "manifest failed signature verification",
		Description: `During manifest upload, if the manifest fails signature
		verification, this error will be returned.`,
		HTTPStatusCodes: []int{http.StatusBadRequest},
	},
	{
		Code:    ErrorCodeBlobUnknown,
		Value:   "BLOB_UNKNOWN",
		Message: "blob unknown to registry",
		Description: `This error may be returned when a blob is unknown to the
		registry in a specified repository. This can be returned with a
		standard get or if a manifest references an unknown layer during
		upload.`,
		HTTPStatusCodes: []int{http.StatusBadRequest, http.StatusNotFound},
	},

	{
		Code:    ErrorCodeBlobUploadUnknown,
		Value:   "BLOB_UPLOAD_UNKNOWN",
		Message: "blob upload unknown to registry",
		Description: `If a blob upload has been cancelled or was never
		started, this error code may be returned.`,
		HTTPStatusCodes: []int{http.StatusNotFound},
	},
}

var errorCodeToDescriptors map[ErrorCode]ErrorDescriptor
var idToDescriptors map[string]ErrorDescriptor

func init() {
	errorCodeToDescriptors = make(map[ErrorCode]ErrorDescriptor, len(ErrorDescriptors))
	idToDescriptors = make(map[string]ErrorDescriptor, len(ErrorDescriptors))

	for _, descriptor := range ErrorDescriptors {
		errorCodeToDescriptors[descriptor.Code] = descriptor
		idToDescriptors[descriptor.Value] = descriptor
	}
}
