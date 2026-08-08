// Package host runs a set of extensions for a host process (such as the Moby
// daemon) and resolves their point providers. It is the public entry point to
// the extension runtime: it wraps the broker and the out-of-process launcher,
// which are implementation details, so a host depends only on this package and
// the public extension contracts.
package host

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/moby/moby/v2/internal/extensions"
	"github.com/moby/moby/v2/internal/extensions/clientpoint"
	"github.com/moby/moby/v2/internal/extensions/internal/broker"
	"github.com/moby/moby/v2/internal/extensions/internal/launcher"
	"github.com/moby/moby/v2/internal/extensions/serverpoint"
	"google.golang.org/grpc"
)

// Options configures a [Host].
type Options struct {
	// RuntimeDir is where per-extension sockets are created.
	RuntimeDir string
	// Extensions are in-process extensions to register.
	Extensions []extensions.Extension
	// Dirs are directories scanned for out-of-process extension binaries. Each
	// executable directly under a dir is launched as an extension named after
	// its file (dir/<id>, or dir/<id>.exe on Windows); extensions live side by
	// side in one directory.
	Dirs []string
	// ClientProviders lists the points the host supports out of process and how
	// to build an in-process caller for each; a point contract's generated wiring
	// exposes one as ClientPoint. It is the boundary on what a launched extension
	// may provide: an extension that declares a point absent from this list is
	// rejected, because the host has no way to call it -- unless the point is
	// listed in ExposeOnlyPoints.
	ClientProviders []clientpoint.Registration
	// ExposeOnlyPoints are points a launched extension may declare that have no
	// in-daemon caller. Listing one exempts it from the ClientProviders rejection
	// above, without wiring an in-daemon provider for it. The host that owns the
	// point's policy can later inspect ServicesForPoint for services registered
	// under that point; service.grpc uses this to publish only opted-in services
	// on the daemon API socket.
	ExposeOnlyPoints []extensions.PointID
	// ExtensionConfig is each extension's configuration keyed by extension id. It
	// is delivered to in-process extensions via Init, and to out-of-process ones
	// over the launch handshake -- so an extension is configured the same way by
	// id wherever it runs.
	ExtensionConfig map[extensions.ExtensionID]extensions.Config
	// DependencyProviders are the points the host makes available to launched
	// extensions as dependencies. The host serves each (backed by the registered
	// provider) on a callback socket that launched extensions reach at init, so an
	// out-of-process extension can call the points it declares a dependency on. A
	// point contract's generated wiring exposes one as ServerPoint.
	DependencyProviders []serverpoint.Registration
}

// Host runs extensions and resolves their point providers. It satisfies
// [extensions.Resolver].
type Host struct {
	broker *broker.Broker
	// conns holds the gRPC connection to each launched out-of-process extension,
	// so the daemon can proxy its socket-exposed services to it.
	conns map[extensions.ExtensionID]grpc.ClientConnInterface
	// launched are the out-of-process extensions, in launch order. The host owns
	// their process teardown (Close): a launched process is already running by
	// the time it is registered, so it is stopped here rather than by the broker,
	// which only tears down what it initialized.
	launched []*launcher.Launched
	// callback serves launched extensions' declared dependencies (the daemon's
	// providers) so they can call them; nil when no dependencies are offered.
	callback *grpc.Server
}

// Conn returns the gRPC connection to a launched out-of-process extension.
// In-process extensions have no connection.
func (h *Host) Conn(extension extensions.ExtensionID) (grpc.ClientConnInterface, bool) {
	conn, ok := h.conns[extension]
	return conn, ok
}

// ServicesForPoint returns, per launched out-of-process extension, the
// fully-qualified gRPC service names it serves for point on its per-extension
// socket. The daemon uses this for points whose services it intentionally
// publishes on the API socket, such as service.grpc. In-process extensions are
// not included -- their services are registered on the daemon's own gRPC server.
func (h *Host) ServicesForPoint(point extensions.PointID) map[extensions.ExtensionID][]string {
	out := make(map[extensions.ExtensionID][]string)
	for _, l := range h.launched {
		if len(l.ProviderServices[point]) > 0 {
			out[l.ID] = l.ProviderServices[point]
		}
	}
	return out
}

// New registers the in-process extensions, launches the binaries, and
// initializes the extensions. On any error it shuts down whatever it started:
// the initialized extensions (via the broker) and every launched process.
//
// A single failing extension -- a binary that will not launch, a conflict, an
// Init error -- fails the whole call, by design: the host loads all-or-nothing
// rather than silently starting with a degraded extension set. The caller
// decides what that means (the daemon treats it as a startup failure).
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
	// Unwind on any failure. The broker shuts down only what it initialized, so
	// launched processes -- already running once launched, before Init -- are
	// stopped here explicitly; they are not the broker's to tear down.
	defer func() {
		if retErr != nil {
			_ = b.Shutdown(context.Background())
			closeLaunched(context.Background(), launched)
			if callback != nil {
				callback.Stop()
			}
		}
	}()

	// The callback socket path is fixed up front so it can go in every launch
	// handshake, even though the server starts only once all providers are
	// registered (below); launched extensions dial it lazily at init.
	callbackEndpoint := ""
	if len(opts.DependencyProviders) > 0 {
		callbackEndpoint = filepath.Join(opts.RuntimeDir, "callback.sock")
	}

	for _, ext := range opts.Extensions {
		if err := b.Register(ext); err != nil {
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

	// Every provider is registered, so serve the offered dependencies on the
	// callback socket before initializing. b.Init then runs Init in dependency
	// order -- for a launched extension, that is its Initialize RPC -- so when it
	// resolves a dependency over the callback, the provider is already initialized.
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

// serveCallback starts the gRPC server that launched extensions reach to call
// their declared dependencies. It serves each offered point backed by the
// provider the broker resolves for it. A point with no provider is skipped
// (nothing depends on what nothing provides), but a point with more than one is
// a real misconfiguration: the callback serves a single provider per point, so
// an ambiguous point would otherwise let a dependent pass init and then get
// Unimplemented at call time. That is failed loudly at startup instead,
// consistent with the host's all-or-nothing loading.
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

// SingleProvider returns the only provider for point.
func (h *Host) SingleProvider(point extensions.PointID) (any, error) {
	return h.broker.SingleProvider(point)
}

// Providers returns all providers for point.
func (h *Host) Providers(point extensions.PointID) []extensions.ResolvedProvider {
	return h.broker.Providers(point)
}

// Shutdown stops every running extension: the initialized extensions in reverse
// dependency order (via the broker), then every launched process, then the
// dependency callback server.
func (h *Host) Shutdown(ctx context.Context) error {
	err := h.broker.Shutdown(ctx)
	err = errors.Join(err, closeLaunchedErr(ctx, h.launched))
	if h.callback != nil {
		h.callback.Stop()
	}
	return err
}

// closeLaunched stops every launched process, ignoring errors. It is the
// best-effort teardown used when host construction fails.
func closeLaunched(ctx context.Context, launched []*launcher.Launched) {
	_ = closeLaunchedErr(ctx, launched)
}

// closeLaunchedErr stops every launched process in reverse launch order,
// joining any errors.
func closeLaunchedErr(ctx context.Context, launched []*launcher.Launched) error {
	var errs []error
	for i := len(launched) - 1; i >= 0; i-- {
		errs = append(errs, launched[i].Close(ctx))
	}
	return errors.Join(errs...)
}

// clientProviderMap indexes the registrations by point id, rejecting duplicates.
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

// extensionFromLaunched builds an extension from a launched out-of-process
// extension, constructing a client-side provider for each point it declares
// from the host's supported providers. A launched extension is pure data, so it
// is a [extensions.Declaration] wrapped with [extensions.New]. It has no
// Shutdown: the host stops the process (see [Host.launched]), not the broker.
func extensionFromLaunched(launched *launcher.Launched, providers map[extensions.PointID]clientpoint.Provider, exposeOnly map[extensions.PointID]bool) (extensions.Extension, error) {
	decl := extensions.Declaration{
		ID:           launched.ID,
		Dependencies: launched.Dependencies,
		Conflicts:    launched.Conflicts,
		// Init runs the extension's own Init in its process, over the Initialize
		// RPC. Wiring it here lets the broker drive it in dependency order along
		// with in-process extensions -- so a launched extension's dependencies are
		// initialized (and reachable over the callback) before it initializes. Its
		// config already arrived over the handshake, so the config the broker would
		// pass is ignored.
		Init: func(ctx context.Context, _ extensions.Config, _ extensions.Resolver) error {
			return launched.Initialize(ctx)
		},
	}
	for _, p := range launched.Points {
		if exposeOnly[p.ID] {
			// An expose-only point (e.g. service.grpc) is not a wire point and is
			// never called in-daemon, so it has no ClientProvider. Any services
			// it registers are recorded under this point in launched.ProviderServices
			// and may be published by the host that owns that policy.
			continue
		}
		build, ok := providers[p.ID]
		if !ok {
			// The daemon has no in-process caller for this point, so it cannot
			// support an extension providing it. A declared point absent from
			// ClientProviders (and not expose-only) is rejected.
			return nil, fmt.Errorf("extension %q declares unsupported point %q", launched.ID, p.ID)
		}
		provider := build(launched.Conn)
		decl.Providers = append(decl.Providers, provider)
	}
	return extensions.New(decl), nil
}
