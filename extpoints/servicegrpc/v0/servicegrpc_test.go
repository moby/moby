package servicegrpcv0

import (
	"testing"

	"github.com/moby/moby/v2/internal/extensions"
	"google.golang.org/grpc"
	"gotest.tools/v3/assert"
)

type fakeResolver struct{ providers []extensions.ResolvedProvider }

func (fakeResolver) Provider(extensions.PointID, extensions.ExtensionID) (any, error) {
	return nil, nil
}
func (fakeResolver) SingleProvider(extensions.PointID) (any, error) { return nil, nil }
func (f fakeResolver) Providers(extensions.PointID) []extensions.ResolvedProvider {
	return f.providers
}

// svcProvider registers one gRPC service named name.
type svcProvider struct{ name string }

func (p svcProvider) RegisterServices(r grpc.ServiceRegistrar) {
	r.RegisterService(&grpc.ServiceDesc{ServiceName: p.name}, nil)
}

// noopProvider exposes nothing.
type noopProvider struct{}

func (noopProvider) RegisterServices(grpc.ServiceRegistrar) {}

// recordRegistrar records the service names registered on it.
type recordRegistrar struct{ names []string }

func (r *recordRegistrar) RegisterService(desc *grpc.ServiceDesc, _ any) {
	r.names = append(r.names, desc.ServiceName)
}

// TestCollect gathers exposed services from every provider, without registering
// them, and skips a provider that exposes nothing.
func TestCollect(t *testing.T) {
	r := fakeResolver{providers: []extensions.ResolvedProvider{
		{Extension: "a", Impl: svcProvider{name: "a.v1.Svc"}},
		{Extension: "b", Impl: noopProvider{}},
	}}
	services, err := Collect(r)
	assert.NilError(t, err)
	assert.Equal(t, len(services), 1)
	assert.Equal(t, services[0].Name, "a.v1.Svc")
}

// TestRegistrarRecordsAndForwards checks the recording registrar both captures
// the names and passes each registration through to its target.
func TestRegistrarRecordsAndForwards(t *testing.T) {
	target := &recordRegistrar{}
	rec := &Registrar{Target: target}
	svcProvider{name: "x.v1.S"}.RegisterServices(rec)
	assert.DeepEqual(t, rec.Names, []string{"x.v1.S"})
	assert.DeepEqual(t, target.names, []string{"x.v1.S"})
}
