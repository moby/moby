package grpcproxy_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/v2/internal/extensions/grpcproxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// streamerDesc is a hand-written server-streaming service test.Streamer/Count
// that reads one request and streams it back three times. There is no generated
// code: the point is to drive the proxy with a streaming method.
var streamerDesc = grpc.ServiceDesc{
	ServiceName: "test.Streamer",
	HandlerType: (*any)(nil),
	Streams: []grpc.StreamDesc{{
		StreamName:    "Count",
		ServerStreams: true,
		Handler: func(_ any, ss grpc.ServerStream) error {
			var req wrapperspb.StringValue
			if err := ss.RecvMsg(&req); err != nil {
				return err
			}
			for i := range 3 {
				if err := ss.SendMsg(wrapperspb.String(fmt.Sprintf("%s-%d", req.GetValue(), i))); err != nil {
					return err
				}
			}
			return nil
		},
	}},
}

// TestProxyServerStreaming checks the proxy forwards a server-streaming method:
// the client opens a stream through the proxy and receives every reply the
// backend streams, in order, with the proxy only forwarding raw frames.
func TestProxyServerStreaming(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	backendSock := filepath.Join(t.TempDir(), "backend.sock")
	backendConn := serve(t, backendSock, func(s *grpc.Server) { s.RegisterService(&streamerDesc, nil) })

	proxy := grpcproxy.New(map[string]grpc.ClientConnInterface{"test.Streamer": backendConn})
	proxySock := filepath.Join(t.TempDir(), "proxy.sock")
	lis, err := net.Listen("unix", proxySock)
	assert.NilError(t, err)
	go proxy.Serve(lis)
	defer proxy.Stop()

	clientConn, err := grpc.NewClient("unix:"+proxySock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NilError(t, err)
	defer clientConn.Close()

	stream, err := clientConn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, "/test.Streamer/Count")
	assert.NilError(t, err)
	assert.NilError(t, stream.SendMsg(wrapperspb.String("hi")))
	assert.NilError(t, stream.CloseSend())

	var got []string
	for {
		var reply wrapperspb.StringValue
		err := stream.RecvMsg(&reply)
		if errors.Is(err, io.EOF) {
			break
		}
		assert.NilError(t, err)
		got = append(got, reply.GetValue())
	}
	assert.DeepEqual(t, got, []string{"hi-0", "hi-1", "hi-2"})
}

// unaryDesc is a hand-written unary service test.Unary/Echo that echoes its
// request back prefixed with "echo:". It has no generated code; the point is to
// drive the proxy with a unary method.
var unaryDesc = grpc.ServiceDesc{
	ServiceName: "test.Unary",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{{
		MethodName: "Echo",
		Handler: func(_ any, _ context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
			var req wrapperspb.StringValue
			if err := dec(&req); err != nil {
				return nil, err
			}
			return wrapperspb.String("echo:" + req.GetValue()), nil
		},
	}},
}

// TestProxyUnary checks the proxy forwards a plain unary call end to end: the
// request reaches the backend and the backend's single reply comes back to the
// client, with the proxy only forwarding raw frames.
func TestProxyUnary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientConn := startProxy(t, "test.Unary", func(s *grpc.Server) { s.RegisterService(&unaryDesc, nil) })

	var reply wrapperspb.StringValue
	err := clientConn.Invoke(ctx, "/test.Unary/Echo", wrapperspb.String("hi"), &reply)
	assert.NilError(t, err)
	assert.Equal(t, reply.GetValue(), "echo:hi")
}

// statusDesc is a hand-written unary service test.Status/Fail whose handler
// always returns a non-OK gRPC status, to check the proxy forwards the
// backend's status code and message unchanged.
var statusDesc = grpc.ServiceDesc{
	ServiceName: "test.Status",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{{
		MethodName: "Fail",
		Handler: func(_ any, _ context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
			var req wrapperspb.StringValue
			if err := dec(&req); err != nil {
				return nil, err
			}
			return nil, status.Error(codes.PermissionDenied, "no")
		},
	}},
}

// TestProxyForwardsBackendStatus checks that a non-OK status returned by the
// backend reaches the client through the proxy with its code and message intact
// -- the proxy must forward the backend's trailer and status, not replace it
// with an Internal error of its own.
func TestProxyForwardsBackendStatus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientConn := startProxy(t, "test.Status", func(s *grpc.Server) { s.RegisterService(&statusDesc, nil) })

	var reply wrapperspb.StringValue
	err := clientConn.Invoke(ctx, "/test.Status/Fail", wrapperspb.String("hi"), &reply)
	assert.Check(t, is.Equal(status.Code(err), codes.PermissionDenied))
	assert.Check(t, is.Equal(status.Convert(err).Message(), "no"))
}

// metaDesc is a hand-written unary service test.Meta/Do whose handler sets both
// response header and trailer metadata, to check the proxy forwards each to the
// client.
var metaDesc = grpc.ServiceDesc{
	ServiceName: "test.Meta",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{{
		MethodName: "Do",
		Handler: func(_ any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
			var req wrapperspb.StringValue
			if err := dec(&req); err != nil {
				return nil, err
			}
			_ = grpc.SetHeader(ctx, metadata.Pairs("x-head", "H"))
			grpc.SetTrailer(ctx, metadata.Pairs("x-trail", "T"))
			return wrapperspb.String("ok"), nil
		},
	}},
}

// TestProxyForwardsMetadata checks the proxy forwards the header and trailer
// metadata the backend sets: forwardClientToServer relays the backend's header
// before the first reply, and forward relays its trailer after the last.
func TestProxyForwardsMetadata(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientConn := startProxy(t, "test.Meta", func(s *grpc.Server) { s.RegisterService(&metaDesc, nil) })

	var hdr, tlr metadata.MD
	var reply wrapperspb.StringValue
	err := clientConn.Invoke(ctx, "/test.Meta/Do", wrapperspb.String("hi"), &reply,
		grpc.Header(&hdr), grpc.Trailer(&tlr))
	assert.NilError(t, err)
	assert.DeepEqual(t, hdr.Get("x-head"), []string{"H"})
	assert.DeepEqual(t, tlr.Get("x-trail"), []string{"T"})
}

// collectorDesc is a hand-written client-streaming service test.Collector/Collect
// that reads every request until the client half-closes, then replies once with
// them joined. It drives the proxy's CloseSend (half-close) path.
var collectorDesc = grpc.ServiceDesc{
	ServiceName: "test.Collector",
	HandlerType: (*any)(nil),
	Streams: []grpc.StreamDesc{{
		StreamName:    "Collect",
		ClientStreams: true,
		Handler: func(_ any, ss grpc.ServerStream) error {
			var vals []string
			for {
				var m wrapperspb.StringValue
				err := ss.RecvMsg(&m)
				if err == nil {
					vals = append(vals, m.GetValue())
					continue
				}
				if errors.Is(err, io.EOF) {
					return ss.SendMsg(wrapperspb.String(strings.Join(vals, ",")))
				}
				return err
			}
		},
	}},
}

// TestProxyClientStreaming checks the proxy forwards a client-streaming method:
// the client sends several requests and half-closes, and the backend's single
// reply comes back. This exercises the s2c EOF branch that half-closes the
// backend via CloseSend so the backend sees end-of-stream and can respond.
func TestProxyClientStreaming(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientConn := startProxy(t, "test.Collector", func(s *grpc.Server) { s.RegisterService(&collectorDesc, nil) })

	stream, err := clientConn.NewStream(ctx, &grpc.StreamDesc{ClientStreams: true}, "/test.Collector/Collect")
	assert.NilError(t, err)
	assert.NilError(t, stream.SendMsg(wrapperspb.String("a")))
	assert.NilError(t, stream.SendMsg(wrapperspb.String("b")))
	assert.NilError(t, stream.SendMsg(wrapperspb.String("c")))
	assert.NilError(t, stream.CloseSend())

	var reply wrapperspb.StringValue
	assert.NilError(t, stream.RecvMsg(&reply))
	assert.Equal(t, reply.GetValue(), "a,b,c")

	// A second Recv drains the backend's clean end-of-stream.
	assert.Check(t, is.ErrorIs(stream.RecvMsg(new(wrapperspb.StringValue)), io.EOF))
}

// startProxy starts a backend gRPC server with register, puts a proxy in front
// of it routing service to that backend, and returns a client connection to the
// proxy. Everything is cleaned up when the test ends.
func startProxy(t *testing.T, service string, register func(*grpc.Server)) *grpc.ClientConn {
	t.Helper()
	backendConn := serve(t, filepath.Join(t.TempDir(), "backend.sock"), register)

	proxy := grpcproxy.New(map[string]grpc.ClientConnInterface{service: backendConn})
	proxySock := filepath.Join(t.TempDir(), "proxy.sock")
	lis, err := net.Listen("unix", proxySock)
	assert.NilError(t, err)
	go proxy.Serve(lis)
	t.Cleanup(proxy.Stop)

	clientConn, err := grpc.NewClient("unix:"+proxySock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NilError(t, err)
	t.Cleanup(func() { clientConn.Close() })
	return clientConn
}

// serve starts a gRPC server on sock, registers services via register, and
// returns a connection to it.
func serve(t *testing.T, sock string, register func(*grpc.Server)) grpc.ClientConnInterface {
	t.Helper()
	s := grpc.NewServer()
	register(s)
	lis, err := net.Listen("unix", sock)
	assert.NilError(t, err)
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	conn, err := grpc.NewClient("unix:"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NilError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}
