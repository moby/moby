package createspecv0

import (
	"context"
	"errors"
	"testing"

	"github.com/moby/moby/v2/internal/extensions"
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

// appendHook appends tag to the spec, so a later provider observing it proves
// the spec was threaded through the earlier one.
type appendHook struct{ tag string }

func (h appendHook) CreateSpec(_ context.Context, req *SpecRequest) (*SpecAdjustment, error) {
	return &SpecAdjustment{Spec: append(append([]byte{}, req.Spec...), h.tag...)}, nil
}
func (appendHook) Validate(context.Context, *SpecRequest) error { return nil }

func TestCreateSpecThreadsSpecInOrder(t *testing.T) {
	r := fakeResolver{providers: []extensions.ResolvedProvider{
		{Extension: "a", Impl: appendHook{"-a"}},
		{Extension: "b", Impl: appendHook{"-b"}},
	}}
	out, err := CreateSpec(context.Background(), r, &SpecRequest{Spec: []byte("spec")})
	assert.NilError(t, err)
	// "-b" appended after "-a" means the second provider saw the first's output.
	assert.Equal(t, string(out), "spec-a-b")
}

type vetoHook struct{}

func (vetoHook) CreateSpec(context.Context, *SpecRequest) (*SpecAdjustment, error) { return nil, nil }
func (vetoHook) Validate(context.Context, *SpecRequest) error {
	return errors.New("spec is not allowed")
}

func TestValidateVetoes(t *testing.T) {
	r := fakeResolver{providers: []extensions.ResolvedProvider{{Extension: "v", Impl: vetoHook{}}}}
	err := Validate(context.Background(), r, &SpecRequest{})
	assert.ErrorContains(t, err, "spec is not allowed")
}
