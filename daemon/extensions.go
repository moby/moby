package daemon

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/containerd/log"
	"github.com/moby/extensions"
	"github.com/moby/extensions/grpcproxy"
	"github.com/moby/extensions/host"
	"github.com/moby/extensions/serverpoint"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/rootless"
	servicegrpcv0 "github.com/moby/moby/v2/extpoints/servicegrpc/v0"
	"github.com/moby/moby/v2/pkg/homedir"
	"google.golang.org/grpc"
)

// newExtensionHost builds the daemon's extension host.
func newExtensionHost(ctx context.Context, cfg *config.Config) (*host.Host, error) {
	return host.New(ctx,
		host.WithRuntimeDir(filepath.Join(cfg.ExecRoot, "extensions")),
		host.WithExtensions(builtinExtensions(cfg)...),
		host.WithDirs(extensionDirs(cfg)...),
		host.WithClientProviders(clientProviders()...),
		host.WithProviderPolicy(host.PointPolicyFunc(func(extensions.ExtensionIdentity, extensions.PointID) host.PointPolicyResult {
			return host.Allow()
		})),
		host.WithDependencyProviders(dependencyProviders()...),
		host.WithExtensionConfig(extensionConfig(cfg)),
	)
}

// dependencyProviders lists points that launched extensions may call back into.
func dependencyProviders() []serverpoint.Registration {
	return nil
}

// extensionConfig converts daemon.json's extension-config entries to host form.
func extensionConfig(cfg *config.Config) map[extensions.ExtensionID]extensions.Config {
	if len(cfg.ExtensionConfig) == 0 {
		return nil
	}
	out := make(map[extensions.ExtensionID]extensions.Config, len(cfg.ExtensionConfig))
	for _, extensionConfig := range cfg.ExtensionConfig {
		out[extensionConfig.ID] = extensionConfig.Config
	}
	return out
}

// extensionDirs are the directories scanned for out-of-process extension
// binaries: the ones configured with --extension-dir, or the default location
// when none are configured.
func extensionDirs(cfg *config.Config) []string {
	if len(cfg.ExtensionDirs) > 0 {
		return cfg.ExtensionDirs
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

// ExposeExtensionServices publishes services selected through service.grpc on
// the API socket. In-process services are registered on gs; out-of-process
// services are proxied by name. Service-name collisions fail startup.
func (daemon *Daemon) ExposeExtensionServices(gs *grpc.Server) (*grpcproxy.Proxy, error) {
	if daemon.extensionHost == nil {
		return nil, nil
	}
	reserved := make(map[string]struct{})
	for name := range gs.GetServiceInfo() {
		reserved[name] = struct{}{}
	}

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

	var backends []grpcproxy.Backend
	for ext, names := range daemon.extensionHost.PublishedServicesForPoint(servicegrpcv0.Point.ID()) {
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
