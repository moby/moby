package sdk

import (
	"errors"
	"fmt"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/extensions/sdk/sdkapi"
	"github.com/moby/extensions/serverpoint"
	"google.golang.org/grpc"
)

// Register adds ext and serves each provider with its matching serverpoint. The
// registrations must cover every point ext provides. The SDK records service
// names by provider point; the daemon decides which services are public.
func (s *Server) Register(ext extensions.Extension, points ...serverpoint.Registration) error {
	if s.registered {
		return errors.New("server already has an extension")
	}
	decl := ext.Declaration()
	if decl.ID == "" {
		return errors.New("extension id is required")
	}
	byPoint := make(map[extensions.PointID]serverpoint.Register, len(points))
	for _, p := range points {
		byPoint[p.Point] = p.Register
	}
	s.declaration.ID = string(decl.ID)
	for _, provider := range decl.Providers {
		register, ok := byPoint[provider.Point]
		if !ok {
			return fmt.Errorf("extension %q: no server registration for point %q", decl.ID, provider.Point)
		}
		rec := &recordingRegistrar{target: s.grpc}
		register(rec, provider.Impl)
		s.declaration.Providers = append(s.declaration.Providers, sdkapi.PointDeclaration{ID: string(provider.Point)})
		s.declaration.ProviderServices = append(s.declaration.ProviderServices, sdkapi.ProviderServices{
			Point:    string(provider.Point),
			Services: rec.names,
		})
	}
	for _, dep := range decl.Dependencies {
		s.declaration.Dependencies = append(s.declaration.Dependencies, sdkapi.Dependency{
			Point:     string(dep.Point),
			Extension: string(dep.Extension),
			Optional:  dep.Optional,
		})
	}
	for _, id := range decl.Conflicts {
		s.declaration.Conflicts = append(s.declaration.Conflicts, string(id))
	}
	s.init = decl.Init
	s.shutdown = decl.Shutdown
	s.registered = true
	return nil
}

// recordingRegistrar records service names while forwarding registrations.
type recordingRegistrar struct {
	target grpc.ServiceRegistrar
	names  []string
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.names = append(r.names, desc.ServiceName)
	r.target.RegisterService(desc, impl)
}

// Depends registers client wiring for points the extension calls through the
// callback channel.
func (s *Server) Depends(regs ...clientpoint.Registration) {
	if s.depends == nil {
		s.depends = make(map[extensions.PointID]clientpoint.Provider, len(regs))
	}
	for _, r := range regs {
		s.depends[r.Point] = r.Provider
	}
}
