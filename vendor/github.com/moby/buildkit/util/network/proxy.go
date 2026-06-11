package network

import (
	"context"
	"io"
	"slices"
	"sync"

	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
)

// ProxyPolicy authorizes requests made through a BuildKit-owned exec proxy.
type ProxyPolicy interface {
	Evaluate(context.Context, *pb.Op) (bool, error)
}

type ProxyConfig struct {
	Policy     ProxyPolicy
	Capture    *ProxyCapture
	EgressMode pb.NetMode
}

type ProxyProvider interface {
	io.Closer
	NewProxy(ctx context.Context, cfg *ProxyConfig) (ProxyNamespace, error)
}

// ProxyNamespace is implemented by network namespaces that expose an internal
// HTTP(S) proxy to the container.
type ProxyNamespace interface {
	Namespace
	ProxyEnv() []string
	ProxyCACert() []byte
}

type ProxyMaterial struct {
	URL    string
	Digest digest.Digest
}

type ProxyRequest struct {
	Method         string
	URL            string
	RedirectTarget string
	StatusCode     int
}

type ProxyIncomplete struct {
	Method string
	URL    string
	Reason string
}

type ProxyCapture struct {
	mu         sync.Mutex
	requests   []ProxyRequest
	materials  []ProxyMaterial
	incomplete []ProxyIncomplete
}

func NewProxyCapture() *ProxyCapture {
	return &ProxyCapture{}
}

func (c *ProxyCapture) AddMaterial(m ProxyMaterial) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.materials = append(c.materials, m)
}

func (c *ProxyCapture) AddRequest(r ProxyRequest) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, r)
}

func (c *ProxyCapture) AddIncomplete(in ProxyIncomplete) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.incomplete = append(c.incomplete, in)
}

func (c *ProxyCapture) Materials() []ProxyMaterial {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := slices.Clone(c.materials)
	redirects := map[string]string{}
	digests := map[string]digest.Digest{}
	for _, m := range out {
		digests[m.URL] = m.Digest
	}
	for _, r := range c.requests {
		if r.RedirectTarget != "" && r.URL != r.RedirectTarget {
			redirects[r.URL] = r.RedirectTarget
		}
	}
	for {
		added := false
		for from, to := range redirects {
			if _, ok := digests[from]; ok {
				continue
			}
			dgst, ok := digests[to]
			if !ok {
				continue
			}
			digests[from] = dgst
			out = append(out, ProxyMaterial{
				URL:    from,
				Digest: dgst,
			})
			added = true
		}
		if !added {
			break
		}
	}
	return out
}

func (c *ProxyCapture) Requests() []ProxyRequest {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ProxyRequest, len(c.requests))
	copy(out, c.requests)
	return out
}

func (c *ProxyCapture) Incomplete() []ProxyIncomplete {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ProxyIncomplete, len(c.incomplete))
	copy(out, c.incomplete)
	return out
}
