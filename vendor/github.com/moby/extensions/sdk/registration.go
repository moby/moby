package sdk

import (
	"errors"
	"fmt"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	servicev0 "github.com/moby/extensions/extpoints/service/v0"
	"github.com/moby/extensions/sdk/sdkapi"
	"github.com/moby/extensions/serverpoint"
	"google.golang.org/grpc"
)

// Register adds ext and serves each ordinary provider with its matching generated
// server registration.
// The service metadata Point records offers and has no transport registration.
func (s *Server) Register(ext extensions.Extension, points ...serverpoint.Registration) error {
	if s.registered {
		return errors.New("server already has an extension")
	}
	decl := ext.Declaration()
	if decl.ID == "" {
		return errors.New("extension id is required")
	}

	byPoint := make(map[extensions.PointID]serverpoint.Register, len(points))
	for _, point := range points {
		if point.Point == "" || point.Register == nil {
			return errors.New("incomplete server registration")
		}
		if _, exists := byPoint[point.Point]; exists {
			return fmt.Errorf("duplicate server registration for point %q", point.Point)
		}
		byPoint[point.Point] = point.Register
	}

	providers := make(map[extensions.PointID]any, len(decl.Providers))
	for _, provider := range decl.Providers {
		if _, exists := providers[provider.Point]; exists {
			return fmt.Errorf("extension %q provides point %q more than once", decl.ID, provider.Point)
		}
		providers[provider.Point] = provider.Impl
	}

	state := registrationState{
		target: s.grpc,
		byName: make(map[string]extensions.PointID),
	}
	services := make(map[extensions.PointID][]string, len(decl.Providers))
	var offered []extensions.PointID
	seenOffers := make(map[extensions.PointID]bool)
	s.declaration.ID = string(decl.ID)
	for _, provider := range decl.Providers {
		s.declaration.Providers = append(s.declaration.Providers, sdkapi.PointDeclaration{ID: string(provider.Point)})
		if provider.Point == servicev0.Point.ID() {
			metadata, ok := provider.Impl.(servicev0.Provider)
			if !ok {
				return fmt.Errorf("extension %q: point %q has incompatible offer metadata", decl.ID, provider.Point)
			}
			for _, point := range metadata.OfferedPoints() {
				if err := extensions.ValidatePointID(point); err != nil {
					return fmt.Errorf("extension %q: invalid offered point: %w", decl.ID, err)
				}
				if point == servicev0.Point.ID() {
					return fmt.Errorf("extension %q: publication metadata point %q cannot offer itself", decl.ID, point)
				}
				if seenOffers[point] {
					return fmt.Errorf("extension %q: point %q is offered more than once", decl.ID, point)
				}
				seenOffers[point] = true
				offered = append(offered, point)
			}
			continue
		}

		register, ok := byPoint[provider.Point]
		if !ok {
			return fmt.Errorf("extension %q: no server registration for point %q", decl.ID, provider.Point)
		}
		recorder := &recordingRegistrar{point: provider.Point, state: &state}
		register(recorder, provider.Impl)
		if state.err != nil {
			return fmt.Errorf("extension %q: %w", decl.ID, state.err)
		}
		services[provider.Point] = recorder.names
		s.declaration.ProviderServices = append(s.declaration.ProviderServices, sdkapi.ProviderServices{
			Point:    string(provider.Point),
			Services: recorder.names,
		})
	}

	for _, point := range offered {
		if _, ok := providers[point]; !ok {
			return fmt.Errorf("extension %q: offered point %q is not implemented", decl.ID, point)
		}
		if len(services[point]) == 0 {
			return fmt.Errorf("extension %q: offered point %q registered no gRPC service", decl.ID, point)
		}
		s.declaration.OfferedPoints = append(s.declaration.OfferedPoints, string(point))
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

// registrationState rejects one gRPC service name being attributed to different
// ordinary Points.
type registrationState struct {
	target grpc.ServiceRegistrar
	byName map[string]extensions.PointID
	err    error
}

// recordingRegistrar records service names under an ordinary provider Point.
type recordingRegistrar struct {
	point extensions.PointID
	state *registrationState
	names []string
}

func (r *recordingRegistrar) RegisterService(desc *grpc.ServiceDesc, impl any) {
	r.names = append(r.names, desc.ServiceName)
	registeredPoint, ok := r.state.byName[desc.ServiceName]
	if ok {
		if registeredPoint != r.point && r.state.err == nil {
			r.state.err = fmt.Errorf("gRPC service %q is attributed to different points %q and %q", desc.ServiceName, registeredPoint, r.point)
		}
		return
	}
	r.state.byName[desc.ServiceName] = r.point
	r.state.target.RegisterService(desc, impl)
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
