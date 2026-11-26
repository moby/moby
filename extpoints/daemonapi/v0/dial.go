package daemonapiv0

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// Host is a placeholder Engine API host for clients that dial with
// DialContext.
// It selects the unix transport and appears in the client's connection
// errors in place of a real socket path.
const Host = "unix:///resolved-by-daemonapi-v0.sock"

// DialContext returns a dial function that connects to the Engine API
// endpoint api reports, ignoring the network and address it is called with.
// Pass it to client.WithDialContext after client.WithHost(Host), because
// WithHost replaces the dial function:
//
//	c, err := client.New(
//		client.WithHost(daemonapiv0.Host),
//		client.WithDialContext(daemonapiv0.DialContext(api)),
//	)
//
// Without WithHost(Host), the client keeps its default host, so its
// DaemonHost and its connection errors name that host rather than the
// endpoint api reports.
//
// It is a temporary shortcut; prefer a specific extension point when one
// covers what the extension needs.
// Resolve api with Point.Single during initialization, after declaring
// Point.Dependency().
// The returned function asks api for the endpoint on its first call, so the
// daemon creates the socket only once an extension actually uses the API.
//
// The daemon serves the API only after it finishes initializing, so API calls
// made before then, including from Init, fail.
func DialContext(api DaemonAPI) func(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &dialer{api: api, lock: make(chan struct{}, 1)}
	return d.dial
}

type dialer struct {
	api DaemonAPI

	// lock is a one-slot semaphore rather than a mutex so that dials waiting
	// on a slow Get still honor their own context.
	lock chan struct{}
	path string
}

func (d *dialer) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	path, err := d.socketPath(ctx)
	if err != nil {
		return nil, err
	}
	var nd net.Dialer
	return nd.DialContext(ctx, "unix", path)
}

func (d *dialer) socketPath(ctx context.Context) (string, error) {
	select {
	case d.lock <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-d.lock }()
	if d.path != "" {
		return d.path, nil
	}
	resp, err := d.api.Get(ctx, &GetRequest{})
	if err != nil {
		return "", fmt.Errorf("get daemon API endpoint: %w", err)
	}
	path, ok := strings.CutPrefix(resp.Host, "unix://")
	if !ok {
		return "", fmt.Errorf("unsupported daemon API endpoint %q", resp.Host)
	}
	d.path = path
	return path, nil
}
