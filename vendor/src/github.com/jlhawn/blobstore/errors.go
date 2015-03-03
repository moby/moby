package blobstore

import (
	"fmt"
)

type errCode int

const (
	errCodeUnexpected errCode = iota
	errCodeBlobNotExists
	errCodeHashNotSupported
	errCodeCannotListBlobsDir
	errCodeCannotOpenBlobInfo
	errCodeCannotDecodeBlobInfo
	errCodeCannotEncodeBlobInfo
	errCodeCannotMakeBlobDir
	errCodeCannotRemoveBlob
	errCodeCannotMakeTempBlobFile
	errCodeCannotCloseTempBlobFile
	errCodeCannotStatTempBlobFile
	errCodeCannotRenameTempBlobFile
	errCodeCannotRemoveTempBlobFile
	errCodeCannotStoreContent
	errCodeNotImplemented
)

var errDescriptions = map[errCode]string{
	errCodeUnexpected:               "unexpected error",
	errCodeBlobNotExists:            "blob does not exist",
	errCodeHashNotSupported:         "hash not supported",
	errCodeCannotListBlobsDir:       "cannot list blobs directory",
	errCodeCannotOpenBlobInfo:       "cannot open blob info file",
	errCodeCannotDecodeBlobInfo:     "cannot decode blob info",
	errCodeCannotEncodeBlobInfo:     "cannot encode blob info",
	errCodeCannotMakeBlobDir:        "cannot make blob directory",
	errCodeCannotRemoveBlob:         "cannot remove blob",
	errCodeCannotMakeTempBlobFile:   "cannot make temporary blob file",
	errCodeCannotCloseTempBlobFile:  "cannot close temporary blob file",
	errCodeCannotStatTempBlobFile:   "cannot stat temporary blob file",
	errCodeCannotRenameTempBlobFile: "cannot rename temporary blob file",
	errCodeCannotRemoveTempBlobFile: "cannot remove temporary blob file",
	errCodeCannotStoreContent:       "cannot store content blob",
	errCodeNotImplemented:           "not implemented",
}

// storeError is the error type which implements blob store Error.
type storeError struct {
	code    errCode
	message string
}

func newError(code errCode, message string) *storeError {
	return &storeError{
		code:    code,
		message: message,
	}
}

// Error returns the string representation of this blob store error.
func (e *storeError) Error() string {
	description, ok := errDescriptions[e.code]
	if !ok {
		description = "blob store error"
	}

	return fmt.Sprintf("%s: %s", description, e.message)
}

// IsBlobNotExists returns whether this error indicates that a blob does not
// exist.
func (e *storeError) IsBlobNotExists() bool {
	return e.code == errCodeBlobNotExists
}
