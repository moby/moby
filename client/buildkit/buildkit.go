package buildkit

import (
	"context"
	"net"

	"github.com/docker/docker/client"
	bkclient "github.com/moby/buildkit/client"
)

// ClientOpts returns a list of buildkit client options which allows the
// caller to create a buildkit client which will connect to the buildkit
// API provided by the daemon.
//
// Example: bkclient.New(ctx, "", ClientOpts(c)...)
func ClientOpts(c client.CommonAPIClient) []bkclient.ClientOpt {
	session := func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
		return c.DialHijack(ctx, "/session", proto, meta)
	}
	grpc := func(ctx context.Context, _ string) (net.Conn, error) {
		return c.DialHijack(ctx, "/grpc", "h2c", nil)
	}

	return []bkclient.ClientOpt{
		bkclient.WithSessionDialer(session),
		bkclient.WithContextDialer(grpc),
	}
}
