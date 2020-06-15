package grpcerrors

import (
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/moby/buildkit/util/stack"
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
		pb := st.Proto()
		pb.Code = int32(Code(err))
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
		if st2, err := st.WithDetails(details...); err == nil {
			st = st2
		}
	}

	return st.Err()
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
		return Code(wrapped.Unwrap())
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
		return AsGRPCStatus(wrapped.Unwrap())
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
		var m interface{}
		detail := &ptypes.DynamicAny{}
		if err := ptypes.UnmarshalAny(d, detail); err != nil {
			detail := &gogotypes.DynamicAny{}
			if err := gogotypes.UnmarshalAny(gogoAny(d), detail); err != nil {
				n.Details = append(n.Details, d)
				continue
			}
			m = detail.Message
		} else {
			m = detail.Message
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

	err = status.FromProto(n).Err()

	for _, s := range stacks {
		if s != nil {
			err = stack.Wrap(err, *s)
		}
	}

	for _, d := range details {
		err = d.WrapError(err)
	}

	return stack.Enable(err)
}

type withCode struct {
	code codes.Code
	error
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
