package session

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func serve(ctx context.Context, grpcServer *grpc.Server, conn net.Conn) {
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	bklog.G(ctx).Debugf("serving grpc connection")
	(&http2.Server{}).ServeConn(conn, &http2.ServeConnOpts{Handler: grpcServer})
}

func grpcClientConn(ctx context.Context, conn net.Conn) (context.Context, *grpc.ClientConn, error) {
	var unary []grpc.UnaryClientInterceptor
	var stream []grpc.StreamClientInterceptor

	var dialCount int64
	dialer := grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		if c := atomic.AddInt64(&dialCount, 1); c > 1 {
			return nil, errors.Errorf("only one connection allowed")
		}
		return conn, nil
	})

	dialOpts := []grpc.DialOption{
		dialer,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		unary = append(unary, filterClient(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(span.TracerProvider()), otelgrpc.WithPropagators(propagators))))
		stream = append(stream, otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(span.TracerProvider()), otelgrpc.WithPropagators(propagators)))
	}

	unary = append(unary, grpcerrors.UnaryClientInterceptor)
	stream = append(stream, grpcerrors.StreamClientInterceptor)

	if len(unary) == 1 {
		dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(unary[0]))
	} else if len(unary) > 1 {
		dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(unary...)))
	}

	if len(stream) == 1 {
		dialOpts = append(dialOpts, grpc.WithStreamInterceptor(stream[0]))
	} else if len(stream) > 1 {
		dialOpts = append(dialOpts, grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(stream...)))
	}

	cc, err := grpc.DialContext(ctx, "localhost", dialOpts...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create grpc client")
	}

	ctx, cancel := context.WithCancel(ctx)
	go monitorHealth(ctx, cc, cancel)

	return ctx, cc, nil
}

func monitorHealth(ctx context.Context, cc *grpc.ClientConn, cancelConn func()) {
	defer cancelConn()
	defer cc.Close()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	healthClient := grpc_health_v1.NewHealthClient(cc)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			_, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
			cancel()
			if err != nil {
				return
			}
		}
	}
}
