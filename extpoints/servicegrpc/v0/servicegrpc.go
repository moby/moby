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

// ServerPoint registers a service.grpc provider on an SDK server and records
// the names that the daemon may publish on the API socket.
var ServerPoint = serverpoint.Registration{
	Point: Point.ID(),
	Register: func(r grpc.ServiceRegistrar, impl any) {
		impl.(Provider).RegisterServices(r)
	},
}

// Service is a collected, not-yet-registered gRPC service.
type Service struct {
	Name string
	Desc *grpc.ServiceDesc
	Impl any
}

// Collect gathers services without registering them, allowing the caller to
// check for name conflicts before serving them.
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

// Registrar wraps a [grpc.ServiceRegistrar] and records registered service names
// while forwarding registrations to Target.
type Registrar struct {
	Target grpc.ServiceRegistrar
	Names  []string
}

// RegisterService records desc's service name and registers it on Target.
func (r *Registrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.Names = append(r.Names, desc.ServiceName)
	r.Target.RegisterService(desc, impl)
}
