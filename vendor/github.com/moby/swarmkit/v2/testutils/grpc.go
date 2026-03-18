package testutils

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorDesc returns the error description of err if it was produced by the rpc system.
// Otherwise, it returns err.Error() or empty string when err is nil.
func ErrorDesc(err error) string {
	if s, ok := status.FromError(err); ok {
		return s.Message()
	}
	return err.Error()
}

// ErrorCode returns the error code for err if it was produced by the rpc system.
// Otherwise, it returns codes.Unknown.
func ErrorCode(err error) codes.Code {
	if s, ok := status.FromError(err); ok {
		return s.Code()
	}
	return codes.Unknown
}
