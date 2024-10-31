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
package errdefs

import (
	"github.com/containerd/errdefs"
)

// Definitions of common error types used throughout containerd. All containerd
// errors returned by most packages will map into one of these errors classes.
// Packages should return errors of these types when they want to instruct a
// client to take a particular action.
//
// These errors map closely to grpc errors.
var (
	ErrUnknown            = errdefs.ErrUnknown
	ErrInvalidArgument    = errdefs.ErrInvalidArgument
	ErrNotFound           = errdefs.ErrNotFound
	ErrAlreadyExists      = errdefs.ErrAlreadyExists
	ErrPermissionDenied   = errdefs.ErrPermissionDenied
	ErrResourceExhausted  = errdefs.ErrResourceExhausted
	ErrFailedPrecondition = errdefs.ErrFailedPrecondition
	ErrConflict           = errdefs.ErrConflict
	ErrNotModified        = errdefs.ErrNotModified
	ErrAborted            = errdefs.ErrAborted
	ErrOutOfRange         = errdefs.ErrOutOfRange
	ErrNotImplemented     = errdefs.ErrNotImplemented
	ErrInternal           = errdefs.ErrInternal
	ErrUnavailable        = errdefs.ErrUnavailable
	ErrDataLoss           = errdefs.ErrDataLoss
	ErrUnauthenticated    = errdefs.ErrUnauthenticated

	IsCanceled           = errdefs.IsCanceled
	IsUnknown            = errdefs.IsUnknown
	IsInvalidArgument    = errdefs.IsInvalidArgument
	IsDeadlineExceeded   = errdefs.IsDeadlineExceeded
	IsNotFound           = errdefs.IsNotFound
	IsAlreadyExists      = errdefs.IsAlreadyExists
	IsPermissionDenied   = errdefs.IsPermissionDenied
	IsResourceExhausted  = errdefs.IsResourceExhausted
	IsFailedPrecondition = errdefs.IsFailedPrecondition
	IsConflict           = errdefs.IsConflict
	IsNotModified        = errdefs.IsNotModified
	IsAborted            = errdefs.IsAborted
	IsOutOfRange         = errdefs.IsOutOfRange
	IsNotImplemented     = errdefs.IsNotImplemented
	IsInternal           = errdefs.IsInternal
	IsUnavailable        = errdefs.IsUnavailable
	IsDataLoss           = errdefs.IsDataLoss
	IsUnauthorized       = errdefs.IsUnauthorized
)
