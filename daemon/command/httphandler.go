package command

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/log"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/buildkit/util/stack"
	"github.com/moby/buildkit/util/tracing"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	"github.com/moby/moby/v2/daemon/internal/otelutil"
)

type httpHandler struct {
	ctx        context.Context
	grpcServer *grpc.Server
	apiServer  http.Handler
}

func newHTTPHandler(ctx context.Context, gs *grpc.Server, apiServer http.Handler) http.Handler {
	return &httpHandler{
		ctx:        ctx,
		grpcServer: gs,
		apiServer:  apiServer,
	}
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
		h.grpcServer.ServeHTTP(w, r)
	} else {
		h.apiServer.ServeHTTP(w, r)
	}
}

func newGRPCServer(ctx context.Context) *grpc.Server {
	tp, _ := otelutil.NewTracerProvider(ctx, false)
	return grpc.NewServer(
		grpc.StatsHandler(tracing.ServerStatsHandler(otelgrpc.WithTracerProvider(tp))),
		grpc.ChainUnaryInterceptor(unaryInterceptor, grpcerrors.UnaryServerInterceptor),
		grpc.StreamInterceptor(grpcerrors.StreamServerInterceptor),
		grpc.MaxRecvMsgSize(defaults.DefaultMaxRecvMsgSize),
		grpc.MaxSendMsgSize(defaults.DefaultMaxSendMsgSize),
	)
}

func unaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, _ error) {
	// This method is used by the clients to send their traces to buildkit so they can be included
	// in the daemon trace and stored in the build history record. This method can not be traced because
	// it would cause an infinite loop.
	if strings.HasSuffix(info.FullMethod, "opentelemetry.proto.collector.trace.v1.TraceService/Export") {
		return handler(ctx, req)
	}

	resp, err := handler(ctx, req)
	if err != nil {
		log.G(ctx).WithError(err).Error(info.FullMethod)
		if log.GetLevel() >= log.DebugLevel {
			_, _ = fmt.Fprintf(os.Stderr, "%+v", stack.Formatter(grpcerrors.FromGRPC(err)))
		}
	}
	return resp, err
}
