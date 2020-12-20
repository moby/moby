package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"net/url"

	"github.com/containerd/containerd/defaults"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client/connhelper"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/grpchijack"
	"github.com/moby/buildkit/util/appdefaults"
	"github.com/moby/buildkit/util/grpcerrors"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Client struct {
	conn *grpc.ClientConn
}

type ClientOpt interface{}

// New returns a new buildkit client. Address can be empty for the system-default address.
func New(ctx context.Context, address string, opts ...ClientOpt) (*Client, error) {
	gopts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(defaults.DefaultMaxRecvMsgSize)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(defaults.DefaultMaxSendMsgSize)),
	}
	needDialer := true
	needWithInsecure := true

	var unary []grpc.UnaryClientInterceptor
	var stream []grpc.StreamClientInterceptor

	for _, o := range opts {
		if _, ok := o.(*withFailFast); ok {
			gopts = append(gopts, grpc.FailOnNonTempDialError(true))
		}
		if credInfo, ok := o.(*withCredentials); ok {
			opt, err := loadCredentials(credInfo)
			if err != nil {
				return nil, err
			}
			gopts = append(gopts, opt)
			needWithInsecure = false
		}
		if wt, ok := o.(*withTracer); ok {
			unary = append(unary, otgrpc.OpenTracingClientInterceptor(wt.tracer, otgrpc.LogPayloads()))
			stream = append(stream, otgrpc.OpenTracingStreamClientInterceptor(wt.tracer))
		}
		if wd, ok := o.(*withDialer); ok {
			gopts = append(gopts, grpc.WithContextDialer(wd.dialer))
			needDialer = false
		}
	}
	if needDialer {
		dialFn, err := resolveDialer(address)
		if err != nil {
			return nil, err
		}
		gopts = append(gopts, grpc.WithContextDialer(dialFn))
	}
	if needWithInsecure {
		gopts = append(gopts, grpc.WithInsecure())
	}
	if address == "" {
		address = appdefaults.Address
	}

	// grpc-go uses a slightly different naming scheme: https://github.com/grpc/grpc/blob/master/doc/naming.md
	// This will end up setting rfc non-complient :authority header to address string (e.g. tcp://127.0.0.1:1234).
	// So, here sets right authority header via WithAuthority DialOption.
	addressURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	gopts = append(gopts, grpc.WithAuthority(addressURL.Host))

	unary = append(unary, grpcerrors.UnaryClientInterceptor)
	stream = append(stream, grpcerrors.StreamClientInterceptor)

	if len(unary) == 1 {
		gopts = append(gopts, grpc.WithUnaryInterceptor(unary[0]))
	} else if len(unary) > 1 {
		gopts = append(gopts, grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(unary...)))
	}

	if len(stream) == 1 {
		gopts = append(gopts, grpc.WithStreamInterceptor(stream[0]))
	} else if len(stream) > 1 {
		gopts = append(gopts, grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(stream...)))
	}

	conn, err := grpc.DialContext(ctx, address, gopts...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to dial %q . make sure buildkitd is running", address)
	}
	c := &Client{
		conn: conn,
	}
	return c, nil
}

func (c *Client) controlClient() controlapi.ControlClient {
	return controlapi.NewControlClient(c.conn)
}

func (c *Client) Dialer() session.Dialer {
	return grpchijack.Dialer(c.controlClient())
}

func (c *Client) Close() error {
	return c.conn.Close()
}

type withFailFast struct{}

func WithFailFast() ClientOpt {
	return &withFailFast{}
}

type withDialer struct {
	dialer func(context.Context, string) (net.Conn, error)
}

func WithContextDialer(df func(context.Context, string) (net.Conn, error)) ClientOpt {
	return &withDialer{dialer: df}
}

type withCredentials struct {
	ServerName string
	CACert     string
	Cert       string
	Key        string
}

// WithCredentials configures the TLS parameters of the client.
// Arguments:
// * serverName: specifies the name of the target server
// * ca:				 specifies the filepath of the CA certificate to use for verification
// * cert:			 specifies the filepath of the client certificate
// * key:				 specifies the filepath of the client key
func WithCredentials(serverName, ca, cert, key string) ClientOpt {
	return &withCredentials{serverName, ca, cert, key}
}

func loadCredentials(opts *withCredentials) (grpc.DialOption, error) {
	ca, err := ioutil.ReadFile(opts.CACert)
	if err != nil {
		return nil, errors.Wrap(err, "could not read ca certificate")
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		return nil, errors.New("failed to append ca certs")
	}

	cfg := &tls.Config{
		ServerName: opts.ServerName,
		RootCAs:    certPool,
	}

	// we will produce an error if the user forgot about either cert or key if at least one is specified
	if opts.Cert != "" || opts.Key != "" {
		cert, err := tls.LoadX509KeyPair(opts.Cert, opts.Key)
		if err != nil {
			return nil, errors.Wrap(err, "could not read certificate/key")
		}
		cfg.Certificates = []tls.Certificate{cert}
		cfg.BuildNameToCertificate()
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(cfg)), nil
}

func WithTracer(t opentracing.Tracer) ClientOpt {
	return &withTracer{t}
}

type withTracer struct {
	tracer opentracing.Tracer
}

func resolveDialer(address string) (func(context.Context, string) (net.Conn, error), error) {
	ch, err := connhelper.GetConnectionHelper(address)
	if err != nil {
		return nil, err
	}
	if ch != nil {
		return ch.ContextDialer, nil
	}
	// basic dialer
	return dialer, nil
}
