package grpcerrors

import (
	"context"
	"log"
	"os"

	"github.com/moby/buildkit/util/stack"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

func UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	resp, err = handler(ctx, req)
	oldErr := err
	if err != nil {
		stack.Helper()
		err = ToGRPC(err)
	}
	if oldErr != nil && err == nil {
		logErr := errors.Wrap(err, "invalid grpc error conversion")
		if os.Getenv("BUILDKIT_DEBUG_PANIC_ON_ERROR") == "1" {
			panic(logErr)
		}
		log.Printf("%v", logErr)
		err = oldErr
	}

	return resp, err
}

func StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := ToGRPC(handler(srv, ss))
	if err != nil {
		stack.Helper()
	}
	return err
}

func UnaryClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	err := FromGRPC(invoker(ctx, method, req, reply, cc, opts...))
	if err != nil {
		stack.Helper()
	}
	return err
}

func StreamClientInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	s, err := streamer(ctx, desc, cc, method, opts...)
	if err != nil {
		stack.Helper()
	}
	return s, ToGRPC(err)
}
