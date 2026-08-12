package servicegrpcv0

import (
	"testing"

	"github.com/moby/moby/v2/internal/extensions/extensionstest"
	"google.golang.org/grpc"
	"gotest.tools/v3/assert"
)

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
	r := extensionstest.Resolver{
		{Extension: "a", Impl: svcProvider{name: "a.v1.Svc"}},
		{Extension: "b", Impl: noopProvider{}},
	}
	services, err := Collect(r)
	assert.NilError(t, err)
	assert.Equal(t, len(services), 1)
	assert.Equal(t, services[0].Name, "a.v1.Svc")
}

func TestRegistrarRecordsAndForwards(t *testing.T) {
	target := &recordRegistrar{}
	rec := &Registrar{Target: target}
	svcProvider{name: "x.v1.S"}.RegisterServices(rec)
	assert.DeepEqual(t, rec.Names, []string{"x.v1.S"})
	assert.DeepEqual(t, target.names, []string{"x.v1.S"})
}
