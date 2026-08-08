// Package servicegrpcv0 is the Moby daemon's raw-gRPC exposure point: an
// extension that implements it publishes some of its own gRPC services on
// docker.sock, so external clients can call them directly.
//
// It is Moby-specific, not part of the framework, because it grafts arbitrary
// gRPC services -- ones the framework has no point for -- onto the daemon's own
// gRPC endpoint. It is the escape hatch for gRPC-specific extensions (e.g. a
// service like buildkit's control API); an extension whose surface is a
// framework point should be published by that point instead, not through here.
//
// Unlike a point, it is not a wire contract: its method takes a
// [grpc.ServiceRegistrar], which cannot cross a gRPC boundary, so it is resolved
// locally. The registrar is supplied by whoever realizes the exposure -- the
// daemon's own gRPC server in-process (services served directly on the socket),
// or the extension's own SDK server out-of-process (services the daemon then
// proxies to by name, without knowing their proto). The same code works either
// way.
package servicegrpcv0

import (
	"github.com/moby/moby/v2/internal/extensions"
	"github.com/moby/moby/v2/internal/extensions/serverpoint"
	"google.golang.org/grpc"
)

// Provider is implemented by an extension to opt onto the socket.
type Provider interface {
	// RegisterServices registers the gRPC services the extension publishes on the
	// socket onto r. The framework decides what r is (see the package doc), so the
	// same code works in-process and out-of-process.
	RegisterServices(r grpc.ServiceRegistrar)
}

// Point is the socket-exposure point.
var Point = extensions.DefinePoint[Provider]("org.mobyproject.extension.service.grpc.v0")

// ServerPoint serves a service.grpc provider's own gRPC services on a server: it
// is the point's [serverpoint.Registration], so an out-of-process extension
// hands it to (*sdk.Server).Register exactly like a wire point hands its
// generated ServerPoint. It forwards to [Provider.RegisterServices], so the SDK
// serves the raw services generically and records their names under this point;
// the daemon publishes only this point's recorded services on the API socket.
var ServerPoint = serverpoint.Registration{
	Point: Point.ID(),
	Register: func(r grpc.ServiceRegistrar, impl any) {
		impl.(Provider).RegisterServices(r)
	},
}

// Service is a gRPC service an extension exposes, collected but not yet
// registered, so a caller can check names for conflicts before serving it.
type Service struct {
	Name string
	Desc *grpc.ServiceDesc
	Impl any
}

// Collect gathers the services every provider the resolver knows about would
// register, without registering them -- so the caller (the daemon, for its
// in-process extensions) can check for name conflicts before serving them on its
// own gRPC server.
func Collect(resolver extensions.Resolver) ([]Service, error) {
	providers, err := Point.All(resolver)
	if err != nil {
		return nil, err
	}
	var c collector
	for _, p := range providers {
		p.Impl.RegisterServices(&c)
	}
	return c.services, nil
}

type collector struct{ services []Service }

func (c *collector) RegisterService(desc *grpc.ServiceDesc, impl any) {
	c.services = append(c.services, Service{Name: desc.ServiceName, Desc: desc, Impl: impl})
}

// Registrar wraps a [grpc.ServiceRegistrar], recording the names of the services
// registered through it while passing them to Target. The SDK uses it to serve
// an out-of-process extension's exposed services on its own server and learn
// their names in one pass.
type Registrar struct {
	Target grpc.ServiceRegistrar
	Names  []string
}

// RegisterService records desc's service name and registers it on Target.
func (r *Registrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.Names = append(r.Names, desc.ServiceName)
	r.Target.RegisterService(desc, impl)
}
