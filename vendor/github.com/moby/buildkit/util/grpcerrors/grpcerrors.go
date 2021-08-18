package grpcerrors

import (
	"encoding/json"
	"errors"

	"github.com/containerd/typeurl"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/golang/protobuf/proto" // nolint:staticcheck
	"github.com/golang/protobuf/ptypes/any"
	"github.com/moby/buildkit/util/stack"
	"github.com/sirupsen/logrus"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TypedError interface {
	ToProto() TypedErrorProto
}

type TypedErrorProto interface {
	proto.Message
	WrapError(error) error
}

func ToGRPC(err error) error {
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
		if st2, err := withDetails(st, details...); err == nil {
			st = st2
		}
	}

	return st.Err()
}

func withDetails(s *status.Status, details ...proto.Message) (*status.Status, error) {
	if s.Code() == codes.OK {
		return nil, errors.New("no error details for status with code OK")
	}
	p := s.Proto()
	for _, detail := range details {
		url, err := typeurl.TypeURL(detail)
		if err != nil {
			logrus.Warnf("ignoring typed error %T: not registered", detail)
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
	return &withCode{error: err, code: code}
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
		m, err := typeurl.UnmarshalAny(gogoAny(d))
		if err != nil {
			continue
		}

		switch v := m.(type) {
		case *stack.Stack:
			stacks = append(stacks, v)
		case TypedErrorProto:
			details = append(details, v)
		default:
			n.Details = append(n.Details, d)
		}
	}

	err = &grpcStatusErr{st: status.FromProto(n)}

	for _, s := range stacks {
		if s != nil {
			err = stack.Wrap(err, *s)
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

type grpcStatusErr struct {
	st *status.Status
}

func (e *grpcStatusErr) Error() string {
	if e.st.Code() == codes.OK || e.st.Code() == codes.Unknown {
		return e.st.Message()
	}
	return e.st.Code().String() + ": " + e.st.Message()
}

func (e *grpcStatusErr) GRPCStatus() *status.Status {
	return e.st
}

type withCode struct {
	code codes.Code
	error
}

func (e *withCode) Code() codes.Code {
	return e.code
}

func (e *withCode) Unwrap() error {
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

func gogoAny(in *any.Any) *gogotypes.Any {
	return &gogotypes.Any{
		TypeUrl: in.TypeUrl,
		Value:   in.Value,
	}
}
