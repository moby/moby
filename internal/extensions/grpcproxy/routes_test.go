package grpcproxy

import (
	"testing"

	"google.golang.org/grpc"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// connStub is a distinct ClientConnInterface value so tests can tell which
// backend a route points at.
type connStub struct{ grpc.ClientConnInterface }

func TestBuildRoutesDisjoint(t *testing.T) {
	a, b := &connStub{}, &connStub{}
	routes, err := BuildRoutes([]Backend{
		{ID: "ext.a", Conn: a, Services: []string{"a.Svc1", "a.Svc2"}},
		{ID: "ext.b", Conn: b, Services: []string{"b.Svc"}},
	}, nil)
	assert.NilError(t, err)
	assert.Equal(t, len(routes), 3)
	assert.Equal(t, routes["a.Svc1"], grpc.ClientConnInterface(a))
	assert.Equal(t, routes["a.Svc2"], grpc.ClientConnInterface(a))
	assert.Equal(t, routes["b.Svc"], grpc.ClientConnInterface(b))
}

func TestBuildRoutesRejectsExtensionConflict(t *testing.T) {
	// Two backends expose the same service. It must be an error, not a silent
	// last-writer-wins override, and the message names both backends in ID order
	// regardless of input order.
	_, err := BuildRoutes([]Backend{
		{ID: "ext.z", Conn: &connStub{}, Services: []string{"shared.Svc"}},
		{ID: "ext.a", Conn: &connStub{}, Services: []string{"shared.Svc"}},
	}, nil)
	assert.ErrorContains(t, err, `"ext.a" and "ext.z" both expose gRPC service "shared.Svc"`)
}

func TestBuildRoutesRejectsReservedConflict(t *testing.T) {
	// A backend cannot expose a name the host already serves (here, a daemon
	// gRPC service), so it cannot shadow it.
	reserved := map[string]struct{}{"moby.buildkit.v1.Control": {}}
	_, err := BuildRoutes([]Backend{
		{ID: "ext.evil", Conn: &connStub{}, Services: []string{"moby.buildkit.v1.Control"}},
	}, reserved)
	assert.ErrorContains(t, err, `backend "ext.evil" cannot expose gRPC service "moby.buildkit.v1.Control"`)
	assert.Check(t, is.Contains(err.Error(), "reserved"))
}

func TestBuildRoutesEmpty(t *testing.T) {
	routes, err := BuildRoutes(nil, nil)
	assert.NilError(t, err)
	assert.Equal(t, len(routes), 0)
}
