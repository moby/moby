package grpcerrors

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/containerd/typeurl/v2"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/moby/buildkit/errdefs"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/stack"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type TypedError interface {
	ToProto() TypedErrorProto
}

type TypedErrorProto interface {
	proto.Message
	WrapError(error) error
}

func ToGRPC(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	st, ok := AsGRPCStatus(err)
	if !ok || st == nil {
		st = status.New(Code(err), err.Error())
	}
	if st.Code() != Code(err) {
		code := Code(err)
		if code == codes.OK {
			code = codes.Unknown
		}
		pb := st.Proto()
		pb.Code = int32(code)
		st = status.FromProto(pb)
	}

	// If the original error was wrapped with more context than the GRPCStatus error,
	// copy the original message to the GRPCStatus error
	if errorHasMoreContext(err, st) {
		pb := st.Proto()
		pb.Message = err.Error()
		st = status.FromProto(pb)
	}

	var details []proto.Message

	for _, st := range stack.Traces(err) {
		details = append(details, st)
	}

	each(err, func(err error) {
		if te, ok := err.(TypedError); ok {
			details = append(details, te.ToProto())
		}
	})

	if len(details) > 0 {
		if st2, err := withDetails(ctx, st, details...); err == nil {
			st = st2
		}
	}

	return st.Err()
}

// errorHasMoreContext checks if the original error provides more context by having
// a different message or additional details than the Status.
func errorHasMoreContext(err error, st *status.Status) bool {
	if errMessage := err.Error(); len(errMessage) > len(st.Message()) {
		// check if the longer message in errMessage is only due to
		// prepending with the status code
		var grpcStatusError *grpcStatusError
		if errors.As(err, &grpcStatusError) {
			return st.Code() != grpcStatusError.st.Code() || st.Message() != grpcStatusError.st.Message()
		}
		return true
	}
	return false
}

func withDetails(ctx context.Context, s *status.Status, details ...proto.Message) (*status.Status, error) {
	if s.Code() == codes.OK {
		return nil, errors.New("no error details for status with code OK")
	}
	p := s.Proto()
	for _, detail := range details {
		url, err := typeurl.TypeURL(detail)
		if err != nil {
			bklog.G(ctx).Warnf("ignoring typed error %T: not registered", detail)
			continue
		}
		dt, err := json.Marshal(detail)
		if err != nil {
			return nil, err
		}
		p.Details = append(p.Details, &any.Any{TypeUrl: url, Value: dt})
	}
	return status.FromProto(p), nil
}

func Code(err error) codes.Code {
	if errdefs.IsInternal(err) {
		if errdefs.IsResourceExhausted(err) {
			return codes.ResourceExhausted
		}
		return codes.Internal
	}

	if se, ok := err.(interface {
		Code() codes.Code
	}); ok {
		return se.Code()
	}

	if se, ok := err.(interface {
		GRPCStatus() *status.Status
	}); ok {
		return se.GRPCStatus().Code()
	}

	wrapped, ok := err.(interface {
		Unwrap() error
	})
	if ok {
		if err := wrapped.Unwrap(); err != nil {
			return Code(err)
		}
	}
	return status.FromContextError(err).Code()
}

func WrapCode(err error, code codes.Code) error {
	return &withCodeError{error: err, code: code}
}

func AsGRPCStatus(err error) (*status.Status, bool) {
	if err == nil {
		return nil, true
	}
	if se, ok := err.(interface {
		GRPCStatus() *status.Status
	}); ok {
		return se.GRPCStatus(), true
	}

	wrapped, ok := err.(interface {
		Unwrap() error
	})
	if ok {
		if err := wrapped.Unwrap(); err != nil {
			return AsGRPCStatus(err)
		}
	}

	return nil, false
}

func FromGRPC(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	pb := st.Proto()

	n := &spb.Status{
		Code:    pb.Code,
		Message: pb.Message,
	}

	details := make([]TypedErrorProto, 0, len(pb.Details))
	stacks := make([]*stack.Stack, 0, len(pb.Details))

	// details that we don't understand are copied as proto
	for _, d := range pb.Details {
		m, err := typeurl.UnmarshalAny(d)
		if err != nil {
			bklog.L.Debugf("failed to unmarshal error detail with type %q: %v", d.GetTypeUrl(), err)
			n.Details = append(n.Details, d)
			continue
		}

		switch v := m.(type) {
		case *stack.Stack:
			stacks = append(stacks, v)
		case TypedErrorProto:
			details = append(details, v)
		default:
			bklog.L.Debugf("unknown detail with type %T", v)
			n.Details = append(n.Details, d)
		}
	}

	err = &grpcStatusError{st: status.FromProto(n)}

	for _, s := range stacks {
		if s != nil {
			err = stack.Wrap(err, s)
		}
	}

	for _, d := range details {
		err = d.WrapError(err)
	}

	if err != nil {
		stack.Helper()
	}

	return stack.Enable(err)
}

type grpcStatusError struct {
	st *status.Status
}

func (e *grpcStatusError) Error() string {
	if e.st.Code() == codes.OK || e.st.Code() == codes.Unknown {
		return e.st.Message()
	}
	return e.st.Code().String() + ": " + e.st.Message()
}

func (e *grpcStatusError) GRPCStatus() *status.Status {
	return e.st
}

type withCodeError struct {
	code codes.Code
	error
}

func (e *withCodeError) Code() codes.Code {
	return e.code
}

func (e *withCodeError) Unwrap() error {
	return e.error
}

func each(err error, fn func(error)) {
	fn(err)
	if wrapped, ok := err.(interface {
		Unwrap() error
	}); ok {
		each(wrapped.Unwrap(), fn)
	}
}
