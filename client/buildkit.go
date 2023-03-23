package client

import (
	"context"
	"net"

	"github.com/moby/buildkit/client"
)

// BuildkitClientOpts returns a list of buildkit client options which allows the
// caller to use to create a buildkit client which will connect to the buildkit
// API provided by the daemon.
//
// Example: bkclient.New(ctx, "", BuildkitClientOpts(c)...)
func BuildkitClientOpts(c CommonAPIClient) []client.ClientOpt {
	session := func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
		return c.DialHijack(ctx, "/session", proto, meta)
	}
	grpc := func(ctx context.Context, _ string) (net.Conn, error) {
		return c.DialHijack(ctx, "/grpc", "h2c", nil)
	}

	return []client.ClientOpt{
		client.WithSessionDialer(session),
		client.WithContextDialer(grpc),
	}
}
