package sdk

import (
	"context"
	"fmt"
	"net"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// resolver returns the callback-backed dependency resolver.
func (s *Server) resolver() (extensions.Resolver, error) {
	if s.callbackEndpoint != "" && s.callbackConn == nil {
		conn, err := grpc.NewClient("unix:"+s.callbackEndpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", s.callbackEndpoint)
			}),
		)
		if err != nil {
			return nil, fmt.Errorf("connect to dependency callback: %w", err)
		}
		s.callbackConn = conn
	}
	return &callbackResolver{conn: s.callbackConn, clients: s.depends}, nil
}

// callbackResolver resolves declared dependencies through the daemon callback.
type callbackResolver struct {
	conn    grpc.ClientConnInterface
	clients map[extensions.PointID]clientpoint.Provider
}

// provider builds a caller for the daemon's provider of point. The callback
// serves one provider per point.
func (r *callbackResolver) provider(point extensions.PointID) (any, error) {
	build, ok := r.clients[point]
	if !ok || r.conn == nil {
		return nil, fmt.Errorf("extension has no resolvable dependency for point %q (declare it with Depends)", point)
	}
	return build(r.conn).Impl, nil
}

func (r *callbackResolver) Provider(point extensions.PointID, _ extensions.ExtensionID) (any, error) {
	// Named selection uses the callback's single provider; by-id selection is not
	// available across the process boundary.
	return r.provider(point)
}

func (r *callbackResolver) Providers(point extensions.PointID) []extensions.ResolvedProvider {
	impl, err := r.provider(point)
	if err != nil {
		return nil
	}
	return []extensions.ResolvedProvider{{Impl: impl}}
}
