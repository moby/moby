// Package servicegrpc adapts published ordinary generated points to the gRPC
// transport without replacing their generated handlers.
package servicegrpc

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/moby/extensions"
	"github.com/moby/extensions/serverpoint"
	"google.golang.org/grpc"
)

// Service is one collected, not-yet-registered generated gRPC service.
type Service struct {
	Point extensions.PointID
	Name  string
	Desc  *grpc.ServiceDesc
	Impl  any
}

// Adapt runs an ordinary Point's generated server registration against impl and
// collects its one gRPC service.
func Adapt(registration serverpoint.Registration, impl any) (Service, error) {
	if registration.Point == "" || registration.Register == nil || isNil(impl) {
		return Service{}, errors.New("servicegrpc: incomplete service registration")
	}
	var collected collector
	registration.Register(&collected, impl)
	if len(collected.services) != 1 {
		return Service{}, fmt.Errorf("servicegrpc: point %q registered %d gRPC services; want exactly 1", registration.Point, len(collected.services))
	}
	service := collected.services[0]
	if service.Desc == nil || service.Desc.ServiceName == "" || isNil(service.Impl) {
		return Service{}, fmt.Errorf("servicegrpc: point %q registered an incomplete gRPC service", registration.Point)
	}
	service.Point = registration.Point
	service.Name = service.Desc.ServiceName
	return service, nil
}

// Register adapts registration and impl and registers the generated gRPC service
// on r.
func Register(r grpc.ServiceRegistrar, registration serverpoint.Registration, impl any) error {
	service, err := Adapt(registration, impl)
	if err != nil {
		return err
	}
	service.Register(r)
	return nil
}

type collector struct {
	services []Service
}

func (c *collector) RegisterService(desc *grpc.ServiceDesc, impl any) {
	c.services = append(c.services, Service{Desc: desc, Impl: impl})
}

// Register installs the collected service on r.
func (s Service) Register(r grpc.ServiceRegistrar) {
	r.RegisterService(s.Desc, s.Impl)
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
