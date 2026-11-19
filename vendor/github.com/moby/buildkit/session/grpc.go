package session

import (
	"context"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type healthCheckConfig struct {
	interval              time.Duration
	defaultTimeout        time.Duration
	failureThreshold      int
	successResetThreshold int
}

var defaultHealthCheckConfig = healthCheckConfig{
	interval:              5 * time.Second,
	defaultTimeout:        15 * time.Second,
	failureThreshold:      2,
	successResetThreshold: 1,
}

const headerSessionHealthCustomTimeout = "X-Buildkit-Session-Health-Custom-Timeout"

func healthCheckConfigFromHeaders(h http.Header) healthCheckConfig {
	cfg := defaultHealthCheckConfig

	if v := h.Get(headerSessionHealthCustomTimeout); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			d := time.Duration(ms) * time.Millisecond
			// Avoid test overrides that are unrealistically aggressive for slower machines.
			d = max(d, time.Second)
			cfg.interval = d
			cfg.defaultTimeout = d
			cfg.failureThreshold = 1
			cfg.successResetThreshold = 1
		}
	}

	return cfg
}

func serve(ctx context.Context, grpcServer *grpc.Server, conn net.Conn) {
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	bklog.G(ctx).Debugf("serving grpc connection")
	// grpc's own transport rather than ServeHTTP: the ServeHTTP transport reads
	// request bodies eagerly into an unbounded buffer, so a stream the handler
	// consumes slowly (an exporter writing to a slow destination) never pushes
	// back on the sender and piles up in memory.
	l := newSingleConnListener(conn)
	go func() {
		<-ctx.Done()
		l.Close()
	}()
	_ = grpcServer.Serve(l)
}

// singleConnListener hands out one established connection, then blocks until
// it is closed, so grpc.Server.Serve can serve a connection it did not accept.
type singleConnListener struct {
	conn      net.Conn
	accept    sync.Once
	closeOnce sync.Once
	done      chan struct{}
}

func newSingleConnListener(conn net.Conn) *singleConnListener {
	return &singleConnListener{conn: conn, done: make(chan struct{})}
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	var c net.Conn
	l.accept.Do(func() { c = &listenerConn{Conn: l.conn, l: l} })
	if c != nil {
		return c, nil
	}
	<-l.done
	return nil, net.ErrClosed
}

func (l *singleConnListener) Close() error {
	l.closeOnce.Do(func() { close(l.done) })
	return nil
}

func (l *singleConnListener) Addr() net.Addr { return l.conn.LocalAddr() }

// listenerConn closes its listener with it, so Serve returns once the session
// connection ends, as ServeConn did.
type listenerConn struct {
	net.Conn
	l *singleConnListener
}

func (c *listenerConn) Close() error {
	err := c.Conn.Close()
	c.l.Close()
	return err
}

func grpcClientConn(ctx context.Context, conn net.Conn, opts map[string][]string) (context.Context, *grpc.ClientConn, error) {
	var dialCount atomic.Int64
	dialer := grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		if c := dialCount.Add(1); c > 1 {
			return nil, errors.New("session connection cannot be re-established")
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
	go monitorHealth(ctx, conn, cc, cancel, healthCheckConfigFromHeaders(http.Header(opts)))

	return ctx, cc, nil
}

func monitorHealth(ctx context.Context, conn net.Conn, cc *grpc.ClientConn, cancelConn func(error), cfg healthCheckConfig) {
	closed := false
	closeConn := func(err error) {
		if closed {
			return
		}
		closed = true
		cancelConn(err)
		cc.Close()
		go conn.Close()
	}
	defer closeConn(errors.WithStack(context.Canceled))
	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()
	healthClient := grpc_health_v1.NewHealthClient(cc)

	consecutiveFailures := 0
	consecutiveSuccessful := 0
	lastHealthcheckDuration := time.Duration(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// This healthcheck can erroneously fail in some instances, such as receiving lots of data in a low-bandwidth scenario or too many concurrent builds.
			// So, this healthcheck is purposely long, and can tolerate some failures on purpose.

			healthcheckStart := time.Now()
			timeout := time.Duration(math.Max(float64(cfg.defaultTimeout), float64(lastHealthcheckDuration)*1.5))
			resultCh := make(chan error, 1)
			go func() {
				checkCtx, cancel := context.WithCancelCause(ctx)
				checkCtx, _ = context.WithTimeoutCause(checkCtx, timeout, errors.WithStack(context.DeadlineExceeded)) //nolint:govet
				_, err := healthClient.Check(checkCtx, &grpc_health_v1.HealthCheckRequest{})
				cancel(errors.WithStack(context.Canceled))
				resultCh <- err
			}()

			var err error
			select {
			case <-ctx.Done():
				return
			case err = <-resultCh:
			case <-time.After(timeout):
				err = errors.WithStack(context.DeadlineExceeded)
			}

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
				consecutiveFailures++
				consecutiveSuccessful = 0
				if consecutiveFailures >= cfg.failureThreshold {
					err = errors.Wrap(err, "session healthcheck failed fatally")
					bklog.G(ctx).WithError(err).Error("healthcheck failed fatally")
					closeConn(err)
					return
				}

				bklog.G(ctx).WithError(err).WithFields(logFields).Warn("healthcheck failed")
			} else {
				consecutiveSuccessful++
				if consecutiveFailures > 0 && consecutiveSuccessful >= cfg.successResetThreshold {
					consecutiveFailures = 0
					bklog.G(ctx).WithFields(logFields).Debug("reset healthcheck failure")
				}
			}

			bklog.G(ctx).WithFields(logFields).Trace("healthcheck completed")
		}
	}
}
