package daemonapiv0

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/moby/moby/client"
)

// placeholderHost only selects the plain unix transport; the client dials the
// endpoint the provider reports.
const placeholderHost = "unix:///daemon-api.sock"

// NewClient returns an Engine API client for the endpoint api reports.
// It is a temporary shortcut; prefer a specific extension point when one
// covers what the extension needs.
// Resolve api with Point.Single during initialization, after declaring
// Point.Dependency().
// The client asks api for the endpoint when it first connects, so the daemon
// creates the socket only once an extension actually uses the API.
// opts are applied after the endpoint, so they may override it.
//
// The daemon serves the API only after it finishes initializing, so API calls
// made before then, including from Init, fail.
// The caller owns the returned client and must close it.
func NewClient(api DaemonAPI, opts ...client.Opt) (*client.Client, error) {
	d := &dialer{api: api}
	return client.New(append([]client.Opt{
		client.WithHost(placeholderHost),
		client.WithDialContext(d.dial),
	}, opts...)...)
}

type dialer struct {
	api DaemonAPI

	mu   sync.Mutex
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
	d.mu.Lock()
	defer d.mu.Unlock()
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
