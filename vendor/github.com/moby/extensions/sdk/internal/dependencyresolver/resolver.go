// Package dependencyresolver resolves providers served through a gRPC
// connection.
package dependencyresolver

import (
	"fmt"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"google.golang.org/grpc"
)

// New returns a resolver that builds one provider for each requested point.
func New(conn grpc.ClientConnInterface, clients map[extensions.PointID]clientpoint.Provider) extensions.Resolver {
	return resolver{conn: conn, clients: clients}
}

type resolver struct {
	conn    grpc.ClientConnInterface
	clients map[extensions.PointID]clientpoint.Provider
}

func (r resolver) provider(point extensions.PointID) (any, error) {
	build, ok := r.clients[point]
	if !ok || r.conn == nil {
		return nil, fmt.Errorf("extension has no resolvable dependency for point %q (declare it with Depends)", point)
	}
	return build(r.conn).Impl, nil
}

func (r resolver) Provider(point extensions.PointID, _ extensions.ExtensionID) (any, error) {
	return r.provider(point)
}

func (r resolver) Providers(point extensions.PointID) []extensions.ResolvedProvider {
	impl, err := r.provider(point)
	if err != nil {
		return nil
	}
	return []extensions.ResolvedProvider{{Impl: impl}}
}
