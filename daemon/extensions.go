package daemon

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/rootless"
	createspecpb "github.com/moby/moby/v2/extpoints/createspec/v0/protogen"
	servicegrpcv0 "github.com/moby/moby/v2/extpoints/servicegrpc/v0"
	"github.com/moby/moby/v2/internal/extensions"
	"github.com/moby/moby/v2/internal/extensions/clientpoint"
	"github.com/moby/moby/v2/internal/extensions/grpcproxy"
	"github.com/moby/moby/v2/internal/extensions/host"
	"github.com/moby/moby/v2/internal/extensions/serverpoint"
	"github.com/moby/moby/v2/pkg/homedir"
	"google.golang.org/grpc"
)

// setupExtensionHost builds the daemon's extension host: its own in-process
// extensions (see [builtinExtensions]), the out-of-process directories it
// launches, and the points it supports across the boundary. The daemon is just
// another host -- a built-in extension registers exactly like a launched one.
func setupExtensionHost(ctx context.Context, cfg *config.Config) (*host.Host, error) {
	return host.New(ctx, host.Options{
		RuntimeDir:          filepath.Join(cfg.ExecRoot, "extensions"),
		Extensions:          builtinExtensions(cfg),
		Dirs:                extensionDirs(cfg),
		ClientProviders:     clientProviders(),
		DependencyProviders: dependencyProviders(),
		ExtensionConfig:     extensionConfig(cfg),
		ExposeOnlyPoints:    []extensions.PointID{servicegrpcv0.Point.ID()},
	})
}

// dependencyProviders are the points the daemon offers launched extensions as
// dependencies: it serves each on the callback channel, backed by the provider
// the broker resolves for it, so an out-of-process extension can call a point it
// declares a dependency on. Add a point's ServerPoint here to expose it as a
// callable dependency; a point with no provider is skipped. It is empty until
// the engine core is exposed as callable points, since there is nothing yet for
// an out-of-process extension to depend on in-daemon.
func dependencyProviders() []serverpoint.Registration {
	return nil
}

// extensionConfig is the daemon's per-extension configuration (the
// "extension-config" key in daemon.json), keyed by extension id, in the host's
// form. The host delivers each extension its entry by id -- to an in-process
// extension at Init, to a launched one over the startup handshake.
func extensionConfig(cfg *config.Config) map[extensions.ExtensionID]extensions.Config {
	if len(cfg.ExtensionConfig) == 0 {
		return nil
	}
	out := make(map[extensions.ExtensionID]extensions.Config, len(cfg.ExtensionConfig))
	for id, c := range cfg.ExtensionConfig {
		out[extensions.ExtensionID(id)] = c
	}
	return out
}

// extensionDirs are the directories scanned for out-of-process extension
// binaries: the ones configured with --extension-dir, or the default location
// when none are configured.
func extensionDirs(cfg *config.Config) []string {
	if len(cfg.Extensions) > 0 {
		return cfg.Extensions
	}
	dir, err := defaultExtensionDir()
	if err != nil {
		// Only rootless without a home directory reaches here; without a default
		// the daemon simply loads no extensions rather than failing to start.
		log.G(context.TODO()).WithError(err).Debug("extensions: no default directory")
		return nil
	}
	return []string{dir}
}

// defaultExtensionDir is the standard location for extension binaries:
// /usr/libexec/docker/moby-extensions, or the rootless equivalent under the
// user's libexec home.
func defaultExtensionDir() (string, error) {
	libexecDir := "/usr/libexec"
	if rootless.RunningWithRootlessKit() {
		var err error
		libexecDir, err = homedir.GetLibexecHome()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(libexecDir, "docker", "moby-extensions"), nil
}

// clientProviders is the list of extension points the daemon supports out of
// process. To let separate-binary extensions implement a new point, add that
// point's ClientPoint to this list. That is all you do here -- one line per
// supported point. Every point's generated wiring exposes its ClientPoint, and
// the host uses it to build an in-process caller from a gRPC connection to the
// extension serving the point.
func clientProviders() []clientpoint.Registration {
	return []clientpoint.Registration{
		createspecpb.ClientPoint,
	}
}

// ExposeExtensionServices publishes on the API socket the gRPC services that
// extensions opted to expose through the service.grpc point. Out-of-process
// services are forwarded by name through a proxy the daemon builds without ever
// importing the extension's generated code; the returned proxy is nil when there
// are none. In-process services are registered directly on gs.
//
// gs is the daemon's own gRPC server, already carrying its built-in services
// (e.g. buildkit). Socket exposure works the same for both extension kinds --
// the service.grpc point injects the service either way -- but the plumbing
// differs by location:
//
//   - an in-process extension's services are registered directly on gs, so they
//     are served on the socket alongside the daemon's own;
//   - an out-of-process extension's services are proxied by name to its
//     connection.
//
// Every exposed name -- in-process, out-of-process, and the daemon's own
// reserved services -- must be distinct: a collision is rejected rather than
// silently overriding, so an extension can never shadow another service, and
// startup fails, consistent with the host's all-or-nothing loading.
func (daemon *Daemon) ExposeExtensionServices(gs *grpc.Server) (*grpcproxy.Proxy, error) {
	if daemon.extensionHost == nil {
		return nil, nil
	}
	// Names the daemon's own gRPC server already serves.
	reserved := make(map[string]struct{})
	for name := range gs.GetServiceInfo() {
		reserved[name] = struct{}{}
	}

	// In-process extensions: collect their services, reject any name that
	// collides with a reserved one or another in-process service, then register
	// the survivors on gs.
	inproc, err := servicegrpcv0.Collect(daemon.extensionHost)
	if err != nil {
		return nil, err
	}
	for _, svc := range inproc {
		if _, taken := reserved[svc.Name]; taken {
			return nil, fmt.Errorf("in-process extension cannot expose gRPC service %q: it is already served", svc.Name)
		}
		reserved[svc.Name] = struct{}{}
	}
	for _, svc := range inproc {
		gs.RegisterService(svc.Desc, svc.Impl)
	}

	// Out-of-process extensions: proxy each by name to its connection, rejecting
	// any that collides with a reserved name (now including the in-process ones)
	// or another extension.
	var backends []grpcproxy.Backend
	for ext, names := range daemon.extensionHost.ServicesForPoint(servicegrpcv0.Point.ID()) {
		if conn, ok := daemon.extensionHost.Conn(ext); ok {
			backends = append(backends, grpcproxy.Backend{ID: string(ext), Conn: conn, Services: names})
		}
	}
	routes, err := grpcproxy.BuildRoutes(backends, reserved)
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, nil
	}
	return grpcproxy.New(routes), nil
}

// builtinExtensions returns the in-process extensions the daemon registers
// itself, selected from the daemon config. Each is an ordinary
// [extensions.Extension] value -- no func init(), no global registry -- so the
// active set is exactly this list, and config reaches each one by id through
// host.Options.ExtensionConfig.
//
// It is currently empty. NRI ([github.com/moby/moby/v2/daemon/extproviders/nri]
// .Extension) is the obvious first built-in, but it stays on the legacy
// daemon/internal/nri path for now: its create-spec bridge does not yet deliver
// container lifecycle events or state sync to plugins, exposes neither `docker
// info` status nor live reload, and still rejects the spec adjustments it cannot
// map (see the package TODOs). Routing NRI through the extension today would
// regress those, so it moves here only once the bridge reaches that parity.
func builtinExtensions(*config.Config) []extensions.Extension {
	return nil
}
