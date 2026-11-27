// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

// Package grpcproxy forwards gRPC calls by service name without importing the
// service's proto. It is used to publish extension services on the daemon socket.
package grpcproxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/mem"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// passthroughCodec forwards messages as raw bytes while retaining the proto
// content type expected by the backend.
type passthroughCodec struct{}

func (passthroughCodec) Name() string { return "proto" }

func (passthroughCodec) Marshal(v any) (mem.BufferSlice, error) {
	bs, ok := v.(mem.BufferSlice)
	if !ok {
		return nil, fmt.Errorf("grpcproxy: cannot marshal %T", v)
	}
	bs.Ref() // gRPC frees the returned slice; keep the caller's reference intact.
	return bs, nil
}

func (passthroughCodec) Unmarshal(data mem.BufferSlice, v any) error {
	dst, ok := v.(*mem.BufferSlice)
	if !ok {
		return fmt.Errorf("grpcproxy: cannot unmarshal into %T", v)
	}
	data.Ref() // data is freed when Unmarshal returns; take our own reference.
	*dst = data
	return nil
}

// Proxy forwards calls for a set of gRPC service names and proto packages.
type Proxy struct {
	routes Routes
	server *grpc.Server
}

// Backend identifies a gRPC backend and the services it serves.
// Packages route every service in each proto package to the backend.
type Backend struct {
	ID       string
	Conn     grpc.ClientConnInterface
	Services []string
	Packages []string
}

// Routes maps gRPC service names and proto packages to backend connections.
// An exact service route takes precedence over its package.
type Routes struct {
	Services map[string]grpc.ClientConnInterface
	Packages map[string]grpc.ClientConnInterface
}

// Len reports the number of routes.
func (r Routes) Len() int {
	return len(r.Services) + len(r.Packages)
}

func (r Routes) lookup(service string) (grpc.ClientConnInterface, bool) {
	if conn, ok := r.Services[service]; ok {
		return conn, true
	}

	conn, ok := r.Packages[packageName(service)]

	return conn, ok
}

// BuildRoutes assembles service and package routes, rejecting conflicts.
// A package conflicts with reserved services and with services of other
// backends inside it.
// No route may reach the SDK runtime package, which every SDK extension serves
// on its connection.
// Backends are processed in ID order for deterministic conflict errors.
func BuildRoutes(backends []Backend, reserved map[string]struct{}) (Routes, error) {
	sorted := append([]Backend(nil), backends...)
	slices.SortFunc(sorted, func(a, b Backend) int { return strings.Compare(a.ID, b.ID) })

	routes := Routes{Services: map[string]grpc.ClientConnInterface{}, Packages: map[string]grpc.ClientConnInterface{}}
	serviceOwner := map[string]string{}
	packageOwner := map[string]string{}

	for _, be := range sorted {
		for _, svc := range be.Services {
			if packageName(svc) == runtimePackage {
				return Routes{}, fmt.Errorf("grpcproxy: backend %q cannot expose gRPC service %q: package %q is the SDK runtime", be.ID, svc, runtimePackage)
			}

			if _, taken := reserved[svc]; taken {
				return Routes{}, fmt.Errorf("grpcproxy: backend %q cannot expose gRPC service %q: that name is reserved by an already-served service", be.ID, svc)
			}

			if other, taken := serviceOwner[svc]; taken {
				return Routes{}, fmt.Errorf("grpcproxy: backends %q and %q both expose gRPC service %q", other, be.ID, svc)
			}

			if other, taken := packageOwner[packageName(svc)]; taken && other != be.ID {
				return Routes{}, fmt.Errorf("grpcproxy: backend %q cannot expose gRPC service %q: backend %q exposes its package", be.ID, svc, other)
			}

			serviceOwner[svc] = be.ID
			routes.Services[svc] = be.Conn
		}

		for _, pkg := range be.Packages {
			if pkg == "" {
				return Routes{}, fmt.Errorf("grpcproxy: backend %q cannot expose an empty package", be.ID)
			}

			if pkg == runtimePackage {
				return Routes{}, fmt.Errorf("grpcproxy: backend %q cannot expose package %q: it is the SDK runtime", be.ID, pkg)
			}

			for svc := range reserved {
				if packageName(svc) == pkg {
					return Routes{}, fmt.Errorf("grpcproxy: backend %q cannot expose package %q: it contains already-served service %q", be.ID, pkg, svc)
				}
			}

			if other, taken := packageOwner[pkg]; taken {
				return Routes{}, fmt.Errorf("grpcproxy: backends %q and %q both expose package %q", other, be.ID, pkg)
			}

			for svc, other := range serviceOwner {
				if other != be.ID && packageName(svc) == pkg {
					return Routes{}, fmt.Errorf("grpcproxy: backend %q cannot expose package %q: backend %q exposes its service %q", be.ID, pkg, other, svc)
				}
			}

			packageOwner[pkg] = be.ID
			routes.Packages[pkg] = be.Conn
		}
	}
	return routes, nil
}

// New builds a proxy that forwards each route to its connection.
func New(routes Routes) *Proxy {
	p := &Proxy{routes: routes}
	p.server = grpc.NewServer(
		grpc.ForceServerCodecV2(passthroughCodec{}),
		grpc.UnknownServiceHandler(p.forward),
	)
	return p
}

// Handles reports whether the proxy forwards the gRPC service.
func (p *Proxy) Handles(service string) bool {
	_, ok := p.routes.lookup(service)
	return ok
}

// ServeHTTP serves the gRPC requests it handles over HTTP/2; this is how the
// daemon dispatches to it from its API server.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.server.ServeHTTP(w, r)
}

// Serve serves the proxy directly on lis (gRPC over the listener), for callers
// that do not multiplex it behind an HTTP server.
func (p *Proxy) Serve(lis net.Listener) error { return p.server.Serve(lis) }

// Stop stops serving.
func (p *Proxy) Stop() { p.server.Stop() }

// forward proxies unary and streaming calls to the backend serving the
// requested service. It forwards raw frames, headers, trailers, and status.
func (p *Proxy) forward(_ any, serverStream grpc.ServerStream) error {
	fullMethod, ok := grpc.MethodFromServerStream(serverStream)
	if !ok {
		return status.Error(codes.Internal, "grpcproxy: no method in stream")
	}

	conn, ok := p.routes.lookup(serviceName(fullMethod))
	if !ok {
		return status.Errorf(codes.Unimplemented, "grpcproxy: no backend for %s", fullMethod)
	}

	ctx, cancel := context.WithCancel(serverStream.Context())
	defer cancel()
	if md, ok := metadata.FromIncomingContext(serverStream.Context()); ok {
		ctx = metadata.NewOutgoingContext(ctx, md.Copy())
	}

	clientStream, err := conn.NewStream(ctx,
		&grpc.StreamDesc{ServerStreams: true, ClientStreams: true},
		fullMethod, grpc.ForceCodecV2(passthroughCodec{}))
	if err != nil {
		return err
	}

	s2c := forwardServerToClient(serverStream, clientStream) // requests to the backend
	c2s := forwardClientToServer(clientStream, serverStream) // responses to the client

	// c2s terminates the RPC. s2c reports once; on EOF it half-closes the backend
	// so server-streaming methods can finish.
	for {
		select {
		case err := <-s2c:
			if !errors.Is(err, io.EOF) {
				// Tear down the backend stream if the client failed or cancelled.
				cancel()
				return status.Errorf(codes.Internal, "grpcproxy: forwarding request: %v", err)
			}
			// Half-close the backend so a server-streaming method can finish.
			_ = clientStream.CloseSend()
		case err := <-c2s:
			// Forward the backend's trailer and status.
			serverStream.SetTrailer(clientStream.Trailer())
			if !errors.Is(err, io.EOF) {
				return err
			}
			return nil
		}
	}
}

// forwardServerToClient pumps request frames to the backend.
func forwardServerToClient(src grpc.ServerStream, dst grpc.ClientStream) <-chan error {
	ret := make(chan error, 1)
	go func() {
		for {
			var frame mem.BufferSlice
			if err := src.RecvMsg(&frame); err != nil {
				ret <- err // io.EOF once the client half-closes
				return
			}
			if err := dst.SendMsg(frame); err != nil {
				frame.Free()
				ret <- err
				return
			}
			frame.Free()
		}
	}()
	return ret
}

// forwardClientToServer pumps response frames to the client and forwards the
// backend header before the first frame.
func forwardClientToServer(src grpc.ClientStream, dst grpc.ServerStream) <-chan error {
	ret := make(chan error, 1)
	go func() {
		md, err := src.Header()
		if err != nil {
			ret <- err
			return
		}
		if err := dst.SendHeader(md); err != nil {
			ret <- err
			return
		}
		for {
			var frame mem.BufferSlice
			if err := src.RecvMsg(&frame); err != nil {
				ret <- err // io.EOF once the backend is done
				return
			}
			if err := dst.SendMsg(frame); err != nil {
				frame.Free()
				ret <- err
				return
			}
			frame.Free()
		}
	}()
	return ret
}

// serviceName extracts the service from a "/pkg.Service/Method" gRPC method.
func serviceName(fullMethod string) string {
	s := strings.TrimPrefix(fullMethod, "/")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[:i]
	}
	return s
}

// runtimePackage is the proto package of the SDK runtime service
// (sdk/sdkapi), which serves the Host's lifecycle calls on every extension
// connection.
const runtimePackage = "moby.extension.runtime.v1"

// packageName extracts the proto package from a "pkg.Service" name.
func packageName(service string) string {
	if i := strings.LastIndex(service, "."); i >= 0 {
		return service[:i]
	}

	return ""
}
