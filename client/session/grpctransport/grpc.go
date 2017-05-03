package grpctransport

import (
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/client/session"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var once sync.Once
var methodRe *regexp.Regexp

func getMethodRe() *regexp.Regexp {
	once.Do(func() {
		methodRe = regexp.MustCompile("^/dockersession.v1.((?i)[a-z0-9_\\.]+)/((?i)[a-z0-9_\\.]+)$")
	})
	return methodRe
}

type grpcTransportFactory struct{}

// New returns new gRPC transport
func New() session.TransportFactory {
	return &grpcTransportFactory{}
}

func (gt *grpcTransportFactory) Name() string {
	return "gRPC"
}

func (gt *grpcTransportFactory) ProtoName() string {
	return "h2c"
}

func (gt *grpcTransportFactory) ToMethodName(id, method string) string {
	return methodURL(id, method)
}
func (gt *grpcTransportFactory) FromMethodName(in string) (string, string, error) {
	matches := getMethodRe().FindAllStringSubmatch(in, -1)
	if len(matches) != 1 || len(matches[0]) != 3 {
		return "", "", errors.Errorf("invalid method name %s", in)
	}
	return matches[0][1], matches[0][2], nil
}

func (gt *grpcTransportFactory) NewHandler(ctx context.Context, conn net.Conn) (session.TransportHandler, error) {
	return newHandler(ctx, conn)
}
func (gt *grpcTransportFactory) NewCaller(ctx context.Context, conn net.Conn) (session.TransportCaller, error) {
	return newCaller(ctx, conn)
}

type callbackSelector struct {
	id     string
	method string
}

type grpcHandler struct {
	conn     net.Conn
	handlers map[callbackSelector]session.HandleFunc
}

func newHandler(ctx context.Context, conn net.Conn) (*grpcHandler, error) {
	gh := &grpcHandler{
		conn:     conn,
		handlers: make(map[callbackSelector]session.HandleFunc),
	}
	return gh, nil
}

func (gh *grpcHandler) Register(id, method string, f session.HandleFunc) error {
	// type HandleFunc func(ctx context.Context, opts map[string][]string, stream Stream) error
	gh.handlers[callbackSelector{id: id, method: method}] = f
	return nil
}

func (gh *grpcHandler) Serve(ctx context.Context) error {
	gs := grpc.NewServer()
	go func() {
		<-ctx.Done()
		gh.conn.Close()
	}()

	services := make(map[string][]callbackSelector)

	for sel := range gh.handlers {
		sname := serviceName(sel.id, sel.method)
		services[sname] = append(services[sname], sel)
	}

	for name, selectors := range services {
		var streams []grpc.StreamDesc
		for _, sel := range selectors {
			func(sel callbackSelector) {
				streams = append(streams, grpc.StreamDesc{
					StreamName: streamName(sel.id, sel.method),
					Handler: func(srv interface{}, stream grpc.ServerStream) error {
						logrus.Debugf("handling %v", sel)
						var s session.Stream
						ctx := stream.Context()
						md, _ := metadata.FromContext(ctx)
						s = stream
						return gh.handlers[sel](ctx, md, &s)
					},
					ServerStreams: true,
					ClientStreams: true,
				})
			}(sel)
		}
		gs.RegisterService(&grpc.ServiceDesc{
			ServiceName: name,
			HandlerType: (*interface{})(nil),
			Methods:     []grpc.MethodDesc{},
			Streams:     streams,
		}, (*interface{})(nil))
	}
	logrus.Debugf("serving grpc connection")
	(&http2.Server{}).ServeConn(gh.conn, &http2.ServeConnOpts{Handler: gs})
	return nil
}

type grpcCaller struct {
	cc *grpc.ClientConn
}

func newCaller(ctx context.Context, conn net.Conn) (*grpcCaller, error) {
	dialOpt := grpc.WithDialer(func(addr string, d time.Duration) (net.Conn, error) {
		return conn, nil
	})

	cc, err := grpc.DialContext(ctx, "", dialOpt, grpc.WithInsecure())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create grpc client")
	}

	gc := &grpcCaller{
		cc: cc,
	}

	go func() {
		<-ctx.Done()
		cc.Close()
	}()

	return gc, nil
}

func (gc *grpcCaller) Call(ctx context.Context, id, method string, opt map[string][]string) (session.Stream, error) {
	desc := &grpc.StreamDesc{
		StreamName:    streamName(id, method),
		Handler:       nil,
		ServerStreams: true,
		ClientStreams: true,
	}

	ctx = metadata.NewContext(ctx, opt)
	s, err := grpc.NewClientStream(ctx, desc, gc.cc, methodURL(id, method))
	return s, err
}

func serviceName(id, method string) string {
	return "dockersession.v1." + id
}
func streamName(id, method string) string {
	return method
}
func methodURL(id, method string) string {
	return "/" + serviceName(id, method) + "/" + streamName(id, method)
}
