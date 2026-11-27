// Package servicegrpcv0 defines the Moby-specific point for publishing an
// extension's own gRPC services on docker.sock.
//
// It is resolved locally because its method takes a [grpc.ServiceRegistrar],
// which cannot cross a gRPC boundary.
// Out-of-process services are registered on the extension server and proxied by
// name by the daemon.
package servicegrpcv0

import (
	"github.com/moby/extensions"
	"github.com/moby/extensions/serverpoint"
	"google.golang.org/grpc"
)

// Provider opts an extension onto the daemon socket.
type Provider interface {
	// RegisterServices registers the extension's services on r.
	RegisterServices(r grpc.ServiceRegistrar)
}

// Point is the socket-exposure point.
var Point = extensions.DefinePoint[Provider]("org.mobyproject.extension.service.grpc.v0")

// ServerPoint registers a service.grpc provider on an SDK server.
var ServerPoint = serverpoint.Registration{
	Point: Point.ID(),
	Register: func(r grpc.ServiceRegistrar, impl any) {
		impl.(Provider).RegisterServices(r)
	},
}
