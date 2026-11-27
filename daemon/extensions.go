package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/containerd/log"
	"github.com/moby/extensions"
	servicev0 "github.com/moby/extensions/extpoints/service/v0"
	"github.com/moby/extensions/grpcproxy"
	"github.com/moby/extensions/host"
	sdkapipb "github.com/moby/extensions/sdk/sdkapi/protogen"
	"github.com/moby/extensions/serverpoint"
	"github.com/moby/extensions/servicegrpc"
	servicegrpcv0 "github.com/moby/moby/extpoints/servicegrpc/v0"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/rootless"
	"github.com/moby/moby/v2/pkg/homedir"
	"google.golang.org/grpc"
)

// newExtensionHost builds the daemon's extension host.
func (daemon *Daemon) newExtensionHost(ctx context.Context) (*host.Host, error) {
	cfg := daemon.Config()
	runtimeDir := extensionRuntimeDir(&cfg)
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return nil, fmt.Errorf("create extension runtime directory: %w", err)
	}
	if err := os.Chmod(runtimeDir, 0o700); err != nil {
		return nil, fmt.Errorf("set permissions on extension runtime directory: %w", err)
	}
	// A crashed daemon leaves its API socket behind. Opening the socket
	// replaces it, but a daemon whose extensions never use the API would
	// otherwise keep the dead socket around.
	if err := os.Remove(extensionAPISocket(&cfg)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.G(ctx).WithError(err).Warn("failed to remove stale extension API socket")
	}
	defaults := slices.Concat(builtinExtensions, daemon.daemonExtensions())
	exts, err := selectBuiltinExtensions(cfg.Extensions, defaults, daemon.optionalExtensions())
	if err != nil {
		return nil, err
	}
	return host.New(ctx,
		host.WithRuntimeDir(runtimeDir),
		host.WithExtensions(exts...),
		host.WithDirs(extensionDirs(&cfg)...),
		host.WithClientProviders(clientProviders()...),
		host.WithProviderPolicy(host.PointPolicyFunc(func(extensions.ExtensionIdentity, extensions.PointID) host.PointPolicyResult {
			return host.Allow()
		})),
		host.WithDependencyProviders(dependencyProviders()...),
		host.WithExtensionConfig(extensionConfig(&cfg)),
	)
}

// extensionRuntimeDir holds the extension sockets.
// It is private to the daemon user.
func extensionRuntimeDir(cfg *config.Config) string {
	return filepath.Join(cfg.ExecRoot, "extensions")
}

// extensionAPISocket is the path of the API socket for extensions.
func extensionAPISocket(cfg *config.Config) string {
	return filepath.Join(extensionRuntimeDir(cfg), "api.sock")
}

// extensionAPIEndpoint provides the daemon.api point.
// It opens the API socket for extensions when an extension first asks for the
// endpoint, so daemons without such extensions expose no extra API endpoint.
// The socket is separate from the configured hosts, which may be TLS-only TCP
// or socket-activated and so unusable by extensions.
func (daemon *Daemon) extensionAPIEndpoint(ctx context.Context) (string, error) {
	cfg := daemon.Config()
	path := extensionAPISocket(&cfg)
	if err := daemon.apiSockets.Open(ctx, path); err != nil {
		return "", err
	}
	return "unix://" + path, nil
}

// StartAPISockets serves h on the API sockets the daemon creates for its own
// consumers, such as extensions.
func (daemon *Daemon) StartAPISockets(h http.Handler) {
	daemon.apiSockets.Start(h)
}

// extensionConfig converts daemon.json's extensions.configs entries to host form.
func extensionConfig(cfg *config.Config) map[extensions.ExtensionID]extensions.Config {
	if len(cfg.Extensions.Configs) == 0 {
		return nil
	}
	out := make(map[extensions.ExtensionID]extensions.Config, len(cfg.Extensions.Configs))
	for _, extensionConfig := range cfg.Extensions.Configs {
		out[extensionConfig.ID] = extensionConfig.Config
	}
	return out
}

// extensionDirs are the directories scanned for out-of-process extension
// binaries: the ones configured with --extension-dir, or the default location
// when none are configured.
func extensionDirs(cfg *config.Config) []string {
	if len(cfg.Extensions.Dirs) > 0 {
		return cfg.Extensions.Dirs
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

// ExposeExtensionServices publishes service.grpc services and points offered
// through service.v0 on the API socket.
// In-process services are registered on gs; out-of-process services are proxied.
// Only the services registered for an offered point are proxied.
// Service-name collisions fail startup.
func (daemon *Daemon) ExposeExtensionServices(ctx context.Context, gs *grpc.Server) (*grpcproxy.Proxy, error) {
	if daemon.extensionHost == nil {
		return nil, nil
	}
	h := daemon.extensionHost
	reserved := make(map[string]struct{})
	for name := range gs.GetServiceInfo() {
		reserved[name] = struct{}{}
	}

	// Only in-process service.grpc providers resolve: the out-of-process ones
	// have no client provider and are proxied below.
	grpcProviders, err := servicegrpcv0.Point.All(h)
	if err != nil {
		return nil, err
	}
	// Services registered directly on gs.
	var local localServices
	for _, p := range grpcProviders {
		p.Impl.RegisterServices(&local)
	}
	offers, err := servicev0.Point.All(h)
	if err != nil {
		return nil, err
	}
	var backends []grpcproxy.Backend
	for _, offer := range offers {
		ext := offer.Identity.ID

		points := offer.Impl.OfferedPoints()
		if conn, ok := h.Conn(ext); ok {
			names, err := offeredServiceNames(ctx, conn, ext, points)
			if err != nil {
				return nil, fmt.Errorf("list offered services of extension %q: %w", ext, err)
			}

			backends = append(backends, grpcproxy.Backend{ID: string(ext), Conn: conn, Services: names})

			continue
		}

		for _, point := range points {
			if point == servicegrpcv0.Point.ID() {
				// Collected above, offered or not.
				continue
			}

			svc, err := offeredInProcessService(h, ext, point)
			if err != nil {
				return nil, err
			}

			local = append(local, svc)
		}
	}

	for _, svc := range local {
		if _, taken := reserved[svc.Name]; taken {
			return nil, fmt.Errorf("in-process extension cannot expose gRPC service %q: it is already served", svc.Name)
		}
		reserved[svc.Name] = struct{}{}
	}
	for _, svc := range local {
		gs.RegisterService(svc.Desc, svc.Impl)
	}

	routes, err := grpcproxy.BuildRoutes(backends, reserved)
	if err != nil {
		return nil, err
	}
	if routes.Len() == 0 {
		return nil, nil
	}
	return grpcproxy.New(routes), nil
}

// The host exposes approved points but not their service inventory.
// Read the process declaration with a deadline because API startup waits for it.
func offeredServiceNames(ctx context.Context, conn grpc.ClientConnInterface, ext extensions.ExtensionID, points []extensions.PointID) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := sdkapipb.NewExtensionClient(conn).Describe(ctx, &sdkapipb.DescribeRequest{})
	if err != nil {
		return nil, err
	}

	decl := resp.GetDeclaration()
	if decl.GetId() != string(ext) {
		return nil, fmt.Errorf("unexpected extension ID %q in declaration", decl.GetId())
	}

	byPoint := make(map[extensions.PointID][]string)

	for _, provider := range decl.GetProviderServices() {
		point := extensions.PointID(provider.GetPoint())
		byPoint[point] = append(byPoint[point], provider.GetServices()...)
	}

	var names []string

	for _, point := range points {
		services := byPoint[point]
		if len(services) == 0 || slices.Contains(services, "") {
			return nil, fmt.Errorf("missing gRPC service name for offered point %q", point)
		}

		names = append(names, services...)
	}

	return names, nil
}

// offeredInProcessService adapts an in-process provider of an offered point to
// the point's generated gRPC service.
func offeredInProcessService(h *host.Host, ext extensions.ExtensionID, point extensions.PointID) (servicegrpc.Service, error) {
	servers := offeredPointServers()
	i := slices.IndexFunc(servers, func(r serverpoint.Registration) bool { return r.Point == point })
	if i < 0 {
		return servicegrpc.Service{}, fmt.Errorf("extension %q offers point %q, which has no in-process server registration", ext, point)
	}
	impl, err := h.Provider(point, ext)
	if err != nil {
		return servicegrpc.Service{}, err
	}
	svc, err := servicegrpc.Adapt(servers[i], impl)
	if err != nil {
		return servicegrpc.Service{}, fmt.Errorf("extension %q: publish point %q: %w", ext, point, err)
	}
	return svc, nil
}

// localServices collects services so their names can be checked before they
// are registered on the daemon's gRPC server.
type localServices []servicegrpc.Service

func (s *localServices) RegisterService(desc *grpc.ServiceDesc, impl any) {
	*s = append(*s, servicegrpc.Service{Name: desc.ServiceName, Desc: desc, Impl: impl})
}
