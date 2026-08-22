// Package host runs extensions and resolves their point providers for a host
// process such as the Moby daemon.
package host

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/extensions/internal/broker"
	"github.com/moby/extensions/internal/launcher"
	"github.com/moby/extensions/serverpoint"
	"google.golang.org/grpc"
)

// Options configures a [Host].
type Options struct {
	// RuntimeDir is where extension sockets are created.
	RuntimeDir string
	// Extensions are in-process extensions to register.
	Extensions []extensions.Extension
	// Dirs are scanned for out-of-process extension binaries.
	Dirs []string
	// ClientProviders lists supported out-of-process points and their client
	// wiring. Unlisted points are rejected unless listed in ExposeOnlyPoints.
	ClientProviders []clientpoint.Registration
	// ExposeOnlyPoints are points with no in-daemon caller. Their services can be
	// inspected with ServicesForPoint; service.grpc uses this for socket exposure.
	ExposeOnlyPoints []extensions.PointID
	// ExtensionConfig holds configuration keyed by extension id.
	ExtensionConfig map[extensions.ExtensionID]extensions.Config
	// DependencyProviders are points launched extensions may call over the
	// callback socket.
	DependencyProviders []serverpoint.Registration
}

// Host runs extensions and resolves their point providers.
type Host struct {
	broker *broker.Broker
	// conns holds connections to launched extensions for socket proxying.
	conns map[extensions.ExtensionID]grpc.ClientConnInterface
	// launched holds out-of-process extensions in launch order.
	launched []*launcher.Launched
	// callback serves launched extensions' declared dependencies.
	callback *grpc.Server
}

// Conn returns the gRPC connection to a launched extension.
func (h *Host) Conn(extension extensions.ExtensionID) (grpc.ClientConnInterface, bool) {
	conn, ok := h.conns[extension]
	return conn, ok
}

// ServicesForPoint returns the service names a launched extension serves for a
// point. In-process services are registered on the daemon's gRPC server.
func (h *Host) ServicesForPoint(point extensions.PointID) map[extensions.ExtensionID][]string {
	out := make(map[extensions.ExtensionID][]string)
	for _, l := range h.launched {
		if len(l.ProviderServices[point]) > 0 {
			out[l.ID] = l.ProviderServices[point]
		}
	}
	return out
}

// New registers, launches, and initializes the configured extensions. It tears
// down anything started when an error occurs; loading is all-or-nothing.
func New(ctx context.Context, opts Options) (_ *Host, retErr error) {
	providers, err := clientProviderMap(opts.ClientProviders)
	if err != nil {
		return nil, err
	}
	exposeOnly := make(map[extensions.PointID]bool, len(opts.ExposeOnlyPoints))
	for _, p := range opts.ExposeOnlyPoints {
		exposeOnly[p] = true
	}
	b := broker.New()
	conns := make(map[extensions.ExtensionID]grpc.ClientConnInterface)
	var launched []*launcher.Launched
	var callback *grpc.Server
	// The broker only shuts down initialized extensions, so construction failures
	// also explicitly close launched processes.
	defer func() {
		if retErr != nil {
			_ = b.Shutdown(context.Background())
			closeLaunched(context.Background(), launched)
			if callback != nil {
				callback.Stop()
			}
		}
	}()

	// Put the callback path in each handshake before starting the server.
	callbackEndpoint := ""
	if len(opts.DependencyProviders) > 0 {
		callbackEndpoint = filepath.Join(opts.RuntimeDir, "callback.sock")
	}

	for _, ext := range opts.Extensions {
		if err := b.RegisterBuiltin(ext); err != nil {
			return nil, err
		}
	}
	l := launcher.Launcher{
		RuntimeDir:       opts.RuntimeDir,
		ExtensionConfig:  opts.ExtensionConfig,
		CallbackEndpoint: callbackEndpoint,
	}
	for _, dir := range opts.Dirs {
		bins, err := launcher.Binaries(ctx, dir)
		if err != nil {
			return nil, err
		}
		for _, bin := range bins {
			started, err := l.Launch(ctx, bin)
			if err != nil {
				return nil, err
			}
			launched = append(launched, started)
			ext, err := extensionFromLaunched(started, providers, exposeOnly)
			if err != nil {
				return nil, err
			}
			if err := b.Register(ext); err != nil {
				return nil, err
			}
			conns[started.ID] = started.Conn
		}
	}

	// Check single-provider constraints across all registered extensions.
	for _, reg := range opts.ClientProviders {
		if !reg.Single {
			continue
		}
		point := reg.Point
		if providers := extensions.EffectiveProviders(b.Providers(point)); len(providers) > 1 {
			ids := make([]string, len(providers))
			for i, p := range providers {
				ids[i] = string(p.Extension)
			}
			return nil, fmt.Errorf("point %q admits a single provider, but extensions %s all provide it", point, strings.Join(ids, ", "))
		}
	}

	// Serve dependencies before initialization so dependency callbacks reach an
	// initialized provider.
	if callbackEndpoint != "" {
		callback, err = serveCallback(callbackEndpoint, opts.DependencyProviders, b)
		if err != nil {
			return nil, err
		}
	}

	if err := b.Init(ctx, opts.ExtensionConfig); err != nil {
		return nil, err
	}
	return &Host{broker: b, conns: conns, launched: launched, callback: callback}, nil
}

// serveCallback starts the server for launched extensions' declared
// dependencies. It rejects ambiguous points because the callback serves one
// provider per point.
func serveCallback(endpoint string, deps []serverpoint.Registration, b *broker.Broker) (*grpc.Server, error) {
	srv := grpc.NewServer()
	for _, dep := range deps {
		providers := b.Providers(dep.Point)
		switch len(providers) {
		case 0:
			continue
		case 1:
			dep.Register(srv, providers[0].Impl)
		default:
			return nil, fmt.Errorf("dependency point %q offered on the callback has %d providers; exactly one is required", dep.Point, len(providers))
		}
	}
	if err := os.Remove(endpoint); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale dependency callback socket: %w", err)
	}
	lis, err := net.Listen("unix", endpoint)
	if err != nil {
		return nil, fmt.Errorf("listen on dependency callback socket: %w", err)
	}
	go srv.Serve(lis)
	return srv, nil
}

// Provider returns one provider for point implemented by extension.
func (h *Host) Provider(point extensions.PointID, extension extensions.ExtensionID) (any, error) {
	return h.broker.Provider(point, extension)
}

// Providers returns all providers for point.
func (h *Host) Providers(point extensions.PointID) []extensions.ResolvedProvider {
	return h.broker.Providers(point)
}

// Shutdown stops extensions in reverse dependency order, then closes any
// process not reached by the broker and the dependency callback server.
func (h *Host) Shutdown(ctx context.Context) error {
	err := h.broker.Shutdown(ctx)
	err = errors.Join(err, closeLaunchedErr(ctx, h.launched))
	if h.callback != nil {
		h.callback.Stop()
	}
	return err
}

// closeLaunched is best-effort teardown for failed host construction.
func closeLaunched(ctx context.Context, launched []*launcher.Launched) {
	_ = closeLaunchedErr(ctx, launched)
}

// closeLaunchedErr stops processes in reverse launch order.
func closeLaunchedErr(ctx context.Context, launched []*launcher.Launched) error {
	var errs []error
	for i := len(launched) - 1; i >= 0; i-- {
		errs = append(errs, launched[i].Close(ctx))
	}
	return errors.Join(errs...)
}

// clientProviderMap indexes registrations by point id.
func clientProviderMap(regs []clientpoint.Registration) (map[extensions.PointID]clientpoint.Provider, error) {
	m := make(map[extensions.PointID]clientpoint.Provider, len(regs))
	for _, r := range regs {
		if _, ok := m[r.Point]; ok {
			return nil, fmt.Errorf("duplicate client provider for point %q", r.Point)
		}
		m[r.Point] = r.Provider
	}
	return m, nil
}

// extensionFromLaunched builds a declaration and client providers from a
// launched extension's handshake data.
func extensionFromLaunched(launched *launcher.Launched, providers map[extensions.PointID]clientpoint.Provider, exposeOnly map[extensions.PointID]bool) (extensions.Extension, error) {
	decl := extensions.Declaration{
		ID:           launched.ID,
		Dependencies: launched.Dependencies,
		Conflicts:    launched.Conflicts,
		// The broker drives the RPC in dependency order. Configuration arrived in
		// the launch handshake, so the broker's config argument is ignored.
		Init: func(ctx context.Context, _ extensions.Config, _ extensions.Resolver) error {
			return launched.Initialize(ctx)
		},
		// Let the broker stop the process in reverse dependency order, keeping its
		// dependencies alive until its Shutdown hook has run.
		Shutdown: launched.Close,
	}
	for _, p := range launched.Points {
		if exposeOnly[p.ID] {
			// Expose-only points are published, not called in-daemon.
			continue
		}
		build, ok := providers[p.ID]
		if !ok {
			// The daemon cannot call an unlisted point.
			return nil, fmt.Errorf("extension %q declares unsupported point %q", launched.ID, p.ID)
		}
		provider := build(launched.Conn)
		decl.Providers = append(decl.Providers, provider)
	}
	return extensions.New(decl), nil
}
