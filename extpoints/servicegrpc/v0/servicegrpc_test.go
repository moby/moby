package servicegrpcv0

import (
	"errors"
	"slices"
	"testing"

	"github.com/moby/extensions"
	"google.golang.org/grpc"
)

type staticResolver struct {
	point     extensions.PointID
	providers []extensions.ResolvedProvider
}

func (staticResolver) Provider(extensions.PointID, extensions.ExtensionID) (any, error) {
	return nil, errors.New("not implemented")
}

func (r staticResolver) Providers(point extensions.PointID) []extensions.ResolvedProvider {
	if point != r.point {
		return nil
	}
	return r.providers
}

type svcProvider struct{ name string }

func (p svcProvider) RegisterServices(r grpc.ServiceRegistrar) {
	r.RegisterService(&grpc.ServiceDesc{ServiceName: p.name}, nil)
}

type noopProvider struct{}

func (noopProvider) RegisterServices(grpc.ServiceRegistrar) {}

type recordRegistrar struct{ names []string }

func (r *recordRegistrar) RegisterService(desc *grpc.ServiceDesc, _ any) {
	r.names = append(r.names, desc.ServiceName)
}

func TestCollect(t *testing.T) {
	r := staticResolver{
		point: Point.ID(),
		providers: []extensions.ResolvedProvider{
			{Impl: svcProvider{name: "a.v1.Svc"}},
			{Impl: noopProvider{}},
		},
	}
	services, err := Collect(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 || services[0].Name != "a.v1.Svc" {
		t.Fatalf("got services %+v, want only a.v1.Svc", services)
	}
}

func TestRegistrarRecordsAndForwards(t *testing.T) {
	target := &recordRegistrar{}
	rec := &Registrar{Target: target}
	svcProvider{name: "x.v1.S"}.RegisterServices(rec)
	want := []string{"x.v1.S"}
	if !slices.Equal(rec.Names, want) {
		t.Errorf("recorded names %q, want %q", rec.Names, want)
	}
	if !slices.Equal(target.names, want) {
		t.Errorf("forwarded names %q, want %q", target.names, want)
	}
}
