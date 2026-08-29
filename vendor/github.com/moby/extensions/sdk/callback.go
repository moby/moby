package sdk

import (
	"context"
	"fmt"
	"net"

	"github.com/moby/extensions"
	"github.com/moby/extensions/sdk/internal/dependencyresolver"
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
	return dependencyresolver.New(s.callbackConn, s.depends), nil
}
