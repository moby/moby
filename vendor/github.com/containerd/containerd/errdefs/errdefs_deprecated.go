/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package errdefs defines the common errors used throughout containerd
// packages.
//
// Use with fmt.Errorf to add context to an error.
//
// To detect an error class, use the IsXXX functions to tell whether an error
// is of a certain type.
//
// The functions ToGRPC and FromGRPC can be used to map server-side and
// client-side errors to the correct types.
//
// Deprecated: use [github.com/containerd/errdefs].
package errdefs

import (
	"github.com/containerd/errdefs"
)

// Definitions of common error types used throughout containerd. All containerd
// errors returned by most packages will map into one of these errors classes.
// Packages should return errors of these types when they want to instruct a
// client to take a particular action.
//
// For the most part, we just try to provide local grpc errors. Most conditions
// map very well to those defined by grpc.
var (
	ErrUnknown            = errdefs.ErrUnknown
	ErrInvalidArgument    = errdefs.ErrInvalidArgument
	ErrNotFound           = errdefs.ErrNotFound
	ErrAlreadyExists      = errdefs.ErrAlreadyExists
	ErrFailedPrecondition = errdefs.ErrFailedPrecondition
	ErrUnavailable        = errdefs.ErrUnavailable
	ErrNotImplemented     = errdefs.ErrNotImplemented
)

// IsInvalidArgument returns true if the error is due to an invalid argument
func IsInvalidArgument(err error) bool {
	return errdefs.IsInvalidArgument(err)
}

// IsNotFound returns true if the error is due to a missing object
func IsNotFound(err error) bool {
	return errdefs.IsNotFound(err)
}

// IsAlreadyExists returns true if the error is due to an already existing
// metadata item
func IsAlreadyExists(err error) bool {
	return errdefs.IsAlreadyExists(err)
}

// IsFailedPrecondition returns true if an operation could not proceed to the
// lack of a particular condition
func IsFailedPrecondition(err error) bool {
	return errdefs.IsFailedPrecondition(err)
}

// IsUnavailable returns true if the error is due to a resource being unavailable
func IsUnavailable(err error) bool {
	return errdefs.IsUnavailable(err)
}

// IsNotImplemented returns true if the error is due to not being implemented
func IsNotImplemented(err error) bool {
	return errdefs.IsNotImplemented(err)
}

// IsCanceled returns true if the error is due to `context.Canceled`.
func IsCanceled(err error) bool {
	return errdefs.IsCanceled(err)
}

// IsDeadlineExceeded returns true if the error is due to
// `context.DeadlineExceeded`.
func IsDeadlineExceeded(err error) bool {
	return errdefs.IsDeadlineExceeded(err)
}

// ToGRPC will attempt to map the backend containerd error into a grpc error,
// using the original error message as a description.
//
// Further information may be extracted from certain errors depending on their
// type.
//
// If the error is unmapped, the original error will be returned to be handled
// by the regular grpc error handling stack.
func ToGRPC(err error) error {
	return errdefs.ToGRPC(err)
}

// ToGRPCf maps the error to grpc error codes, assembling the formatting string
// and combining it with the target error string.
//
// This is equivalent to errdefs.ToGRPC(fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err))
func ToGRPCf(err error, format string, args ...interface{}) error {
	return errdefs.ToGRPCf(err, format, args...)
}

// FromGRPC returns the underlying error from a grpc service based on the grpc error code
func FromGRPC(err error) error {
	return errdefs.FromGRPC(err)
}
