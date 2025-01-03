package session

import (
	"context"
	"math"
	"net"
	"sync/atomic"
	"time"

	"github.com/containerd/containerd/v2/defaults"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/buildkit/util/tracing"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(defaults.DefaultMaxRecvMsgSize)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(defaults.DefaultMaxSendMsgSize)),
		grpc.WithUnaryInterceptor(grpcerrors.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(grpcerrors.StreamClientInterceptor),
	}

	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		statsHandler := tracing.ClientStatsHandler(
			otelgrpc.WithTracerProvider(span.TracerProvider()),
			otelgrpc.WithPropagators(propagators),
		)
		dialOpts = append(dialOpts, grpc.WithStatsHandler(statsHandler))
	}

	//nolint:staticcheck // ignore SA1019 NewClient is preferred but has different behavior
	cc, err := grpc.DialContext(ctx, "localhost", dialOpts...)
	if err != nil {
		return ctx, nil, errors.Wrap(err, "failed to create grpc client")
	}

	ctx, cancel := context.WithCancelCause(ctx)
	go monitorHealth(ctx, cc, cancel)

	return ctx, cc, nil
}

func monitorHealth(ctx context.Context, cc *grpc.ClientConn, cancelConn func(error)) {
	defer cancelConn(errors.WithStack(context.Canceled))
	defer cc.Close()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	healthClient := grpc_health_v1.NewHealthClient(cc)

	failedBefore := false
	consecutiveSuccessful := 0
	defaultHealthcheckDuration := 30 * time.Second
	lastHealthcheckDuration := time.Duration(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// This healthcheck can erroneously fail in some instances, such as receiving lots of data in a low-bandwidth scenario or too many concurrent builds.
			// So, this healthcheck is purposely long, and can tolerate some failures on purpose.

			healthcheckStart := time.Now()

			timeout := time.Duration(math.Max(float64(defaultHealthcheckDuration), float64(lastHealthcheckDuration)*1.5))

			ctx, cancel := context.WithCancelCause(ctx)
			ctx, _ = context.WithTimeoutCause(ctx, timeout, errors.WithStack(context.DeadlineExceeded))
			_, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
			cancel(errors.WithStack(context.Canceled))

			lastHealthcheckDuration = time.Since(healthcheckStart)
			logFields := logrus.Fields{
				"timeout":        timeout,
				"actualDuration": lastHealthcheckDuration,
			}

			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if failedBefore {
					bklog.G(ctx).Error("healthcheck failed fatally")
					return
				}

				failedBefore = true
				consecutiveSuccessful = 0
				bklog.G(ctx).WithFields(logFields).Warn("healthcheck failed")
			} else {
				consecutiveSuccessful++

				if consecutiveSuccessful >= 5 && failedBefore {
					failedBefore = false
					bklog.G(ctx).WithFields(logFields).Debug("reset healthcheck failure")
				}
			}

			bklog.G(ctx).WithFields(logFields).Trace("healthcheck completed")
		}
	}
}
