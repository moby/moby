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

// Package errgrpc provides utility functions for translating errors to
// and from a gRPC context.
//
// The functions ToGRPC and ToNative can be used to map server-side and
// client-side errors to the correct types.
package errgrpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/internal/cause"
)

// ToGRPC will attempt to map the backend containerd error into a grpc error,
// using the original error message as a description.
//
// Further information may be extracted from certain errors depending on their
// type.
//
// If the error is unmapped, the original error will be returned to be handled
// by the regular grpc error handling stack.
func ToGRPC(err error) error {
	if err == nil {
		return nil
	}

	if isGRPCError(err) {
		// error has already been mapped to grpc
		return err
	}

	switch {
	case errdefs.IsInvalidArgument(err):
		return status.Error(codes.InvalidArgument, err.Error())
	case errdefs.IsNotFound(err):
		return status.Error(codes.NotFound, err.Error())
	case errdefs.IsAlreadyExists(err):
		return status.Error(codes.AlreadyExists, err.Error())
	case errdefs.IsFailedPrecondition(err) || errdefs.IsConflict(err) || errdefs.IsNotModified(err):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errdefs.IsUnavailable(err):
		return status.Error(codes.Unavailable, err.Error())
	case errdefs.IsNotImplemented(err):
		return status.Error(codes.Unimplemented, err.Error())
	case errdefs.IsCanceled(err):
		return status.Error(codes.Canceled, err.Error())
	case errdefs.IsDeadlineExceeded(err):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case errdefs.IsUnauthorized(err):
		return status.Error(codes.Unauthenticated, err.Error())
	case errdefs.IsPermissionDenied(err):
		return status.Error(codes.PermissionDenied, err.Error())
	case errdefs.IsInternal(err):
		return status.Error(codes.Internal, err.Error())
	case errdefs.IsDataLoss(err):
		return status.Error(codes.DataLoss, err.Error())
	case errdefs.IsAborted(err):
		return status.Error(codes.Aborted, err.Error())
	case errdefs.IsOutOfRange(err):
		return status.Error(codes.OutOfRange, err.Error())
	case errdefs.IsResourceExhausted(err):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errdefs.IsUnknown(err):
		return status.Error(codes.Unknown, err.Error())
	}

	return err
}

// ToGRPCf maps the error to grpc error codes, assembling the formatting string
// and combining it with the target error string.
//
// This is equivalent to grpc.ToGRPC(fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err))
func ToGRPCf(err error, format string, args ...interface{}) error {
	return ToGRPC(fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err))
}

// ToNative returns the underlying error from a grpc service based on the grpc error code
func ToNative(err error) error {
	if err == nil {
		return nil
	}

	desc := errDesc(err)

	var cls error // divide these into error classes, becomes the cause

	switch code(err) {
	case codes.InvalidArgument:
		cls = errdefs.ErrInvalidArgument
	case codes.AlreadyExists:
		cls = errdefs.ErrAlreadyExists
	case codes.NotFound:
		cls = errdefs.ErrNotFound
	case codes.Unavailable:
		cls = errdefs.ErrUnavailable
	case codes.FailedPrecondition:
		if desc == errdefs.ErrConflict.Error() || strings.HasSuffix(desc, ": "+errdefs.ErrConflict.Error()) {
			cls = errdefs.ErrConflict
		} else if desc == errdefs.ErrNotModified.Error() || strings.HasSuffix(desc, ": "+errdefs.ErrNotModified.Error()) {
			cls = errdefs.ErrNotModified
		} else {
			cls = errdefs.ErrFailedPrecondition
		}
	case codes.Unimplemented:
		cls = errdefs.ErrNotImplemented
	case codes.Canceled:
		cls = context.Canceled
	case codes.DeadlineExceeded:
		cls = context.DeadlineExceeded
	case codes.Aborted:
		cls = errdefs.ErrAborted
	case codes.Unauthenticated:
		cls = errdefs.ErrUnauthenticated
	case codes.PermissionDenied:
		cls = errdefs.ErrPermissionDenied
	case codes.Internal:
		cls = errdefs.ErrInternal
	case codes.DataLoss:
		cls = errdefs.ErrDataLoss
	case codes.OutOfRange:
		cls = errdefs.ErrOutOfRange
	case codes.ResourceExhausted:
		cls = errdefs.ErrResourceExhausted
	default:
		if idx := strings.LastIndex(desc, cause.UnexpectedStatusPrefix); idx > 0 {
			if status, err := strconv.Atoi(desc[idx+len(cause.UnexpectedStatusPrefix):]); err == nil && status >= 200 && status < 600 {
				cls = cause.ErrUnexpectedStatus{Status: status}
			}
		}
		if cls == nil {
			cls = errdefs.ErrUnknown
		}
	}

	msg := rebaseMessage(cls, desc)
	if msg != "" {
		err = fmt.Errorf("%s: %w", msg, cls)
	} else {
		err = cls
	}

	return err
}

// rebaseMessage removes the repeats for an error at the end of an error
// string. This will happen when taking an error over grpc then remapping it.
//
// Effectively, we just remove the string of cls from the end of err if it
// appears there.
func rebaseMessage(cls error, desc string) string {
	clss := cls.Error()
	if desc == clss {
		return ""
	}

	return strings.TrimSuffix(desc, ": "+clss)
}

func isGRPCError(err error) bool {
	_, ok := status.FromError(err)
	return ok
}

func code(err error) codes.Code {
	if s, ok := status.FromError(err); ok {
		return s.Code()
	}
	return codes.Unknown
}

func errDesc(err error) string {
	if s, ok := status.FromError(err); ok {
		return s.Message()
	}
	return err.Error()
}
