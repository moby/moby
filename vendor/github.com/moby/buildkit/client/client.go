package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/url"
	"os"
	"time"

	contentapi "github.com/containerd/containerd/api/services/content/v1"
	"github.com/containerd/containerd/v2/defaults"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client/connhelper"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/grpchijack"
	"github.com/moby/buildkit/util/appdefaults"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/buildkit/util/tracing"
	"github.com/moby/buildkit/util/tracing/otlptracegrpc"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn          *grpc.ClientConn
	sessionDialer func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error)
}

type ClientOpt interface {
	isClientOpt()
}

// New returns a new buildkit client. Address can be empty for the system-default address.
func New(ctx context.Context, address string, opts ...ClientOpt) (*Client, error) {
	gopts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(defaults.DefaultMaxRecvMsgSize)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(defaults.DefaultMaxSendMsgSize)),
	}
	needDialer := true

	var customTracer bool // allows manually setting disabling tracing even if tracer in context
	var tracerProvider trace.TracerProvider
	var tracerDelegate TracerDelegate
	var sessionDialer func(context.Context, string, map[string][]string) (net.Conn, error)
	var customDialOptions []grpc.DialOption
	var creds *withCredentials

	for _, o := range opts {
		if credInfo, ok := o.(*withCredentials); ok {
			if creds == nil {
				creds = &withCredentials{}
			}
			creds = creds.merge(credInfo)
		}
		if wt, ok := o.(*withTracer); ok {
			customTracer = true
			tracerProvider = wt.tp
		}
		if wd, ok := o.(*withDialer); ok {
			gopts = append(gopts, grpc.WithContextDialer(wd.dialer))
			needDialer = false
		}
		if wt, ok := o.(*withTracerDelegate); ok {
			tracerDelegate = wt
		}
		if sd, ok := o.(*withSessionDialer); ok {
			sessionDialer = sd.dialer
		}
		if opt, ok := o.(*withGRPCDialOption); ok {
			customDialOptions = append(customDialOptions, opt.opt)
		}
	}

	if creds == nil {
		gopts = append(gopts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		credOpts, err := loadCredentials(creds)
		if err != nil {
			return nil, err
		}
		gopts = append(gopts, credOpts)
	}

	if !customTracer {
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			tracerProvider = span.TracerProvider()
		}
	}

	if tracerProvider != nil {
		gopts = append(gopts, grpc.WithStatsHandler(
			tracing.ClientStatsHandler(
				otelgrpc.WithTracerProvider(tracerProvider),
				otelgrpc.WithPropagators(
					propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
				),
			),
		))
	}

	if needDialer {
		dialFn, err := resolveDialer(address)
		if err != nil {
			return nil, err
		}
		if dialFn != nil {
			gopts = append(gopts, grpc.WithContextDialer(dialFn))
		}
	}
	if address == "" {
		address = appdefaults.Address
	}
	uri, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	// Setting :authority pseudo header
	// - HTTP/2 (RFC7540) defines :authority pseudo header includes
	//   the authority portion of target URI but it must not include
	//   userinfo part (i.e. url.Host).
	//   ref: https://datatracker.ietf.org/doc/html/rfc7540#section-8.1.2.3
	// - However, when TLS specified, grpc-go requires it must match
	//   with its servername specified for certificate validation.
	var authority string
	if creds != nil && creds.serverName != "" {
		authority = creds.serverName
	}
	if authority == "" {
		// authority as hostname from target address
		authority = uri.Host
	}
	if uri.Scheme == "tcp" {
		// remove tcp scheme from address, since default dialer doesn't expect that
		// name resolution is done by grpc according to the following spec: https://github.com/grpc/grpc/blob/master/doc/naming.md
		address = uri.Host
	}

	gopts = append(gopts, grpc.WithAuthority(authority))
	gopts = append(gopts, grpc.WithUnaryInterceptor(grpcerrors.UnaryClientInterceptor))
	gopts = append(gopts, grpc.WithStreamInterceptor(grpcerrors.StreamClientInterceptor))
	gopts = append(gopts, customDialOptions...)

	//nolint:staticcheck // ignore SA1019 NewClient has different behavior and needs to be tested
	conn, err := grpc.DialContext(ctx, address, gopts...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to dial %q . make sure buildkitd is running", address)
	}

	c := &Client{
		conn:          conn,
		sessionDialer: sessionDialer,
	}

	if tracerDelegate != nil {
		_ = c.setupDelegatedTracing(ctx, tracerDelegate) // ignore error
	}

	return c, nil
}

func (c *Client) setupDelegatedTracing(ctx context.Context, td TracerDelegate) error {
	pd := otlptracegrpc.NewClient(c.conn)
	e, err := otlptrace.New(ctx, pd)
	if err != nil {
		return nil
	}
	return td.SetSpanExporter(ctx, e)
}

func (c *Client) ControlClient() controlapi.ControlClient {
	return controlapi.NewControlClient(c.conn)
}

func (c *Client) ContentClient() contentapi.ContentClient {
	return contentapi.NewContentClient(c.conn)
}

func (c *Client) Dialer() session.Dialer {
	return grpchijack.Dialer(c.ControlClient())
}

func (c *Client) Wait(ctx context.Context) error {
	for {
		_, err := c.ControlClient().Info(ctx, &controlapi.InfoRequest{})
		if err == nil {
			return nil
		}

		switch code := grpcerrors.Code(err); code {
		case codes.Unavailable:
		case codes.Unimplemented:
			// only buildkit v0.11+ supports the info api, but an unimplemented
			// response error is still a response so we can ignore it
			return nil
		default:
			return err
		}

		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-time.After(time.Second):
		}
		c.conn.ResetConnectBackoff()
	}
}

func (c *Client) Close() error {
	return c.conn.Close()
}

type withDialer struct {
	dialer func(context.Context, string) (net.Conn, error)
}

func (*withDialer) isClientOpt() {}

func WithContextDialer(df func(context.Context, string) (net.Conn, error)) ClientOpt {
	return &withDialer{dialer: df}
}

type withCredentials struct {
	// server options
	serverName   string
	caCert       string
	caCertSystem bool

	// client options
	cert string
	key  string
}

func (opts *withCredentials) merge(opts2 *withCredentials) *withCredentials {
	result := *opts
	if opts2 == nil {
		return &result
	}

	// server options
	if opts2.serverName != "" {
		result.serverName = opts2.serverName
	}
	if opts2.caCert != "" {
		result.caCert = opts2.caCert
	}
	if opts2.caCertSystem {
		result.caCertSystem = opts2.caCertSystem
	}

	// client options
	if opts2.cert != "" {
		result.cert = opts2.cert
	}
	if opts2.key != "" {
		result.key = opts2.key
	}

	return &result
}

func (*withCredentials) isClientOpt() {}

// WithCredentials configures the TLS parameters of the client.
// Arguments:
// * cert:	specifies the filepath of the client certificate
// * key:	specifies the filepath of the client key
func WithCredentials(cert, key string) ClientOpt {
	return &withCredentials{
		cert: cert,
		key:  key,
	}
}

// WithServerConfig configures the TLS parameters to connect to the server.
// Arguments:
// * serverName:	specifies the server name to verify the hostname
// * caCert:		specifies the filepath of the CA certificate
func WithServerConfig(serverName, caCert string) ClientOpt {
	return &withCredentials{
		serverName: serverName,
		caCert:     caCert,
	}
}

// WithServerConfigSystem configures the TLS parameters to connect to the
// server, using the system's certificate pool.
func WithServerConfigSystem(serverName string) ClientOpt {
	return &withCredentials{
		serverName:   serverName,
		caCertSystem: true,
	}
}

func loadCredentials(opts *withCredentials) (grpc.DialOption, error) {
	cfg := &tls.Config{}

	if opts.caCertSystem {
		cfg.RootCAs, _ = x509.SystemCertPool()
	}
	if cfg.RootCAs == nil {
		cfg.RootCAs = x509.NewCertPool()
	}

	if opts.caCert != "" {
		ca, err := os.ReadFile(opts.caCert)
		if err != nil {
			return nil, errors.Wrap(err, "could not read ca certificate")
		}
		if ok := cfg.RootCAs.AppendCertsFromPEM(ca); !ok {
			return nil, errors.New("failed to append ca certs")
		}
	}

	if opts.serverName != "" {
		cfg.ServerName = opts.serverName
	}

	// we will produce an error if the user forgot about either cert or key if at least one is specified
	if opts.cert != "" || opts.key != "" {
		cert, err := tls.LoadX509KeyPair(opts.cert, opts.key)
		if err != nil {
			return nil, errors.Wrap(err, "could not read certificate/key")
		}
		cfg.Certificates = append(cfg.Certificates, cert)
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(cfg)), nil
}

func WithTracerProvider(t trace.TracerProvider) ClientOpt {
	return &withTracer{t}
}

type withTracer struct {
	tp trace.TracerProvider
}

func (w *withTracer) isClientOpt() {}

type TracerDelegate interface {
	SetSpanExporter(context.Context, sdktrace.SpanExporter) error
}

func WithTracerDelegate(td TracerDelegate) ClientOpt {
	return &withTracerDelegate{
		TracerDelegate: td,
	}
}

type withTracerDelegate struct {
	TracerDelegate
}

func (w *withTracerDelegate) isClientOpt() {}

func WithSessionDialer(dialer func(context.Context, string, map[string][]string) (net.Conn, error)) ClientOpt {
	return &withSessionDialer{dialer}
}

type withSessionDialer struct {
	dialer func(context.Context, string, map[string][]string) (net.Conn, error)
}

func (w *withSessionDialer) isClientOpt() {}

func resolveDialer(address string) (func(context.Context, string) (net.Conn, error), error) {
	ch, err := connhelper.GetConnectionHelper(address)
	if err != nil {
		return nil, err
	}
	if ch != nil {
		return ch.ContextDialer, nil
	}
	return nil, nil
}

type withGRPCDialOption struct {
	opt grpc.DialOption
}

func (*withGRPCDialOption) isClientOpt() {}

func WithGRPCDialOption(opt grpc.DialOption) ClientOpt {
	return &withGRPCDialOption{opt}
}
