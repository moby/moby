package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rootless-containers/rootlesskit/v2/pkg/api"
	"github.com/rootless-containers/rootlesskit/v2/pkg/httputil"
	"github.com/rootless-containers/rootlesskit/v2/pkg/port"
)

type Client interface {
	HTTPClient() *http.Client
	PortManager() port.Manager
	Info(context.Context) (*api.Info, error)
}

// New creates a client.
// socketPath is a path to the UNIX socket, without unix:// prefix.
func New(socketPath string) (Client, error) {
	hc, err := httputil.NewHTTPClient(socketPath)
	if err != nil {
		return nil, err
	}
	return NewWithHTTPClient(hc), nil
}

func NewWithHTTPClient(hc *http.Client) Client {
	return &client{
		Client:    hc,
		version:   "v1",
		dummyHost: "rootlesskit",
	}
}

type client struct {
	*http.Client
	// version is always "v1"
	// TODO(AkihiroSuda): negotiate the version
	version   string
	dummyHost string
}

func (c *client) HTTPClient() *http.Client {
	return c.Client
}

func (c *client) PortManager() port.Manager {
	return &portManager{
		client: c,
	}
}

func (c *client) Info(ctx context.Context) (*api.Info, error) {
	u := fmt.Sprintf("http://%s/%s/info", c.dummyHost, c.version)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := c.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := httputil.Successful(resp); err != nil {
		return nil, err
	}
	var info api.Info
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

type portManager struct {
	*client
}

func (pm *portManager) AddPort(ctx context.Context, spec port.Spec) (*port.Status, error) {
	m, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("http://%s/%s/ports", pm.client.dummyHost, pm.client.version)
	req, err := http.NewRequest("POST", u, bytes.NewReader(m))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)
	resp, err := pm.client.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := httputil.Successful(resp); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(resp.Body)
	var status port.Status
	if err := dec.Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}
func (pm *portManager) ListPorts(ctx context.Context) ([]port.Status, error) {
	u := fmt.Sprintf("http://%s/%s/ports", pm.client.dummyHost, pm.client.version)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := pm.client.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := httputil.Successful(resp); err != nil {
		return nil, err
	}
	var statuses []port.Status
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}
func (pm *portManager) RemovePort(ctx context.Context, id int) error {
	u := fmt.Sprintf("http://%s/%s/ports/%d", pm.client.dummyHost, pm.client.version, id)
	req, err := http.NewRequest("DELETE", u, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	resp, err := pm.client.HTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := httputil.Successful(resp); err != nil {
		return err
	}
	return nil
}
