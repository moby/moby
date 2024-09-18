package resolver

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	distreference "github.com/distribution/reference"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver/pb"
	log "github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/version"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// DefaultPool is the default shared resolver pool instance
var DefaultPool = NewPool()

// Pool is a cache of recently used resolvers
type Pool struct {
	mu sync.Mutex
	m  map[string]*authHandlerNS
}

// NewPool creates a new pool for caching resolvers
func NewPool() *Pool {
	p := &Pool{
		m: map[string]*authHandlerNS{},
	}
	time.AfterFunc(5*time.Minute, p.gc)
	return p
}

func (p *Pool) gc() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for k, ns := range p.m {
		ns.muHandlers.Lock()
		for key, h := range ns.handlers {
			if time.Since(h.lastUsed) < 10*time.Minute {
				continue
			}
			parts := strings.SplitN(key, "/", 2)
			if len(parts) != 2 {
				delete(ns.handlers, key)
				continue
			}
			c, err := ns.sm.Get(context.TODO(), parts[1], true)
			if c == nil || err != nil {
				delete(ns.handlers, key)
			}
		}
		if len(ns.handlers) == 0 {
			delete(p.m, k)
		}
		ns.muHandlers.Unlock()
	}

	time.AfterFunc(5*time.Minute, p.gc)
}

// Clear deletes currently cached items. This may be called on config changes for example.
func (p *Pool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m = map[string]*authHandlerNS{}
}

// GetResolver gets a resolver for a specified scope from the pool
func (p *Pool) GetResolver(hosts docker.RegistryHosts, ref, scope string, sm *session.Manager, g session.Group) *Resolver {
	name := ref
	named, err := distreference.ParseNormalizedNamed(ref)
	if err == nil {
		name = named.Name()
	}

	var key string
	if strings.Contains(scope, "push") {
		// When scope includes "push", index the authHandlerNS cache by session
		// id(s) as well to prevent tokens with potential write access to third
		// party registries from leaking between client sessions. The key will end
		// up looking something like:
		// 'wujskoey891qc5cv1edd3yj3p::repository:foo/bar::pull,push'
		key = fmt.Sprintf("%s::%s::%s", strings.Join(session.AllSessionIDs(g), ":"), name, scope)
	} else {
		// The authHandlerNS is not isolated for pull-only scopes since LLB
		// verticies from pulls all end up in the cache anyway and all
		// requests/clients have access to the same cache
		key = fmt.Sprintf("%s::%s", name, scope)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	h, ok := p.m[key]

	if !ok {
		h = newAuthHandlerNS(sm)
		p.m[key] = h
	}

	log.G(context.TODO()).WithFields(logrus.Fields{
		"name":   name,
		"scope":  scope,
		"key":    key,
		"cached": ok,
	}).Debugf("checked for cached auth handler namespace")

	return newResolver(hosts, h, sm, g)
}

func newResolver(hosts docker.RegistryHosts, handler *authHandlerNS, sm *session.Manager, g session.Group) *Resolver {
	if hosts == nil {
		hosts = docker.ConfigureDefaultRegistries(
			docker.WithClient(newDefaultClient()),
			docker.WithPlainHTTP(docker.MatchLocalhost),
		)
	}
	r := &Resolver{
		hosts:   hosts,
		sm:      sm,
		g:       g,
		handler: handler,
	}
	headers := http.Header{}
	headers.Set("User-Agent", version.UserAgent())
	r.Resolver = docker.NewResolver(docker.ResolverOptions{
		Hosts:   r.HostsFunc,
		Headers: headers,
	})
	return r
}

// Resolver is a wrapper around remotes.Resolver
type Resolver struct {
	remotes.Resolver
	hosts   docker.RegistryHosts
	sm      *session.Manager
	g       session.Group
	handler *authHandlerNS
	auth    *dockerAuthorizer

	is   images.Store
	mode ResolveMode
}

// HostsFunc implements registry configuration of this Resolver
func (r *Resolver) HostsFunc(host string) ([]docker.RegistryHost, error) {
	return func(domain string) ([]docker.RegistryHost, error) {
		v, err := r.handler.g.Do(context.TODO(), domain, func(ctx context.Context) ([]docker.RegistryHost, error) {
			// long lock not needed because flightcontrol.Do
			r.handler.muHosts.Lock()
			v, ok := r.handler.hosts[domain]
			r.handler.muHosts.Unlock()
			if ok {
				return v, nil
			}
			res, err := r.hosts(domain)
			if err != nil {
				return nil, err
			}
			r.handler.muHosts.Lock()
			r.handler.hosts[domain] = res
			r.handler.muHosts.Unlock()
			return res, nil
		})
		if err != nil || v == nil {
			return nil, err
		}
		if len(v) == 0 {
			return nil, nil
		}
		// make a copy so authorizer is set on unique instance
		res := make([]docker.RegistryHost, len(v))
		copy(res, v)
		auth := newDockerAuthorizer(res[0].Client, r.handler, r.sm, r.g)
		for i := range res {
			res[i].Authorizer = auth
		}
		return res, nil
	}(host)
}

// WithSession returns a new resolver that works with new session group
func (r *Resolver) WithSession(s session.Group) *Resolver {
	r2 := *r
	r2.auth = nil
	r2.g = s
	r2.Resolver = docker.NewResolver(docker.ResolverOptions{
		Hosts: r2.HostsFunc, // this refers to the newly-configured session so we need to recreate the resolver.
	})
	return &r2
}

// WithImageStore returns new resolver that can also resolve from local images store
func (r *Resolver) WithImageStore(is images.Store, mode ResolveMode) *Resolver {
	r2 := *r
	r2.Resolver = r.Resolver
	r2.is = is
	r2.mode = mode
	return &r2
}

// Fetcher returns a new fetcher for the provided reference.
func (r *Resolver) Fetcher(ctx context.Context, ref string) (remotes.Fetcher, error) {
	if atomic.LoadInt64(&r.handler.counter) == 0 {
		r.Resolve(ctx, ref)
	}
	return r.Resolver.Fetcher(ctx, ref)
}

// Resolve attempts to resolve the reference into a name and descriptor.
func (r *Resolver) Resolve(ctx context.Context, ref string) (string, ocispecs.Descriptor, error) {
	if r.mode == ResolveModePreferLocal && r.is != nil {
		if img, err := r.is.Get(ctx, ref); err == nil {
			return ref, img.Target, nil
		}
	}

	n, desc, err := r.Resolver.Resolve(ctx, ref)
	if err == nil {
		atomic.AddInt64(&r.handler.counter, 1)
		return n, desc, nil
	}

	if r.mode == ResolveModeDefault && r.is != nil {
		if img, err := r.is.Get(ctx, ref); err == nil {
			return ref, img.Target, nil
		}
	}

	return "", ocispecs.Descriptor{}, err
}

type ResolveMode int

const (
	ResolveModeDefault ResolveMode = iota
	ResolveModeForcePull
	ResolveModePreferLocal
)

func (r ResolveMode) String() string {
	switch r {
	case ResolveModeDefault:
		return pb.AttrImageResolveModeDefault
	case ResolveModeForcePull:
		return pb.AttrImageResolveModeForcePull
	case ResolveModePreferLocal:
		return pb.AttrImageResolveModePreferLocal
	default:
		return ""
	}
}

func ParseImageResolveMode(v string) (ResolveMode, error) {
	switch v {
	case pb.AttrImageResolveModeDefault, "":
		return ResolveModeDefault, nil
	case pb.AttrImageResolveModeForcePull:
		return ResolveModeForcePull, nil
	case pb.AttrImageResolveModePreferLocal:
		return ResolveModePreferLocal, nil
	default:
		return 0, errors.Errorf("invalid resolvemode: %s", v)
	}
}
