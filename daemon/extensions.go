package daemon

import (
	"context"
	"path/filepath"

	"github.com/containerd/log"
	"github.com/moby/extensions"
	"github.com/moby/extensions/host"
	"github.com/moby/extensions/serverpoint"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/rootless"
	"github.com/moby/moby/v2/pkg/homedir"
)

// setupExtensionHost builds the daemon's extension host.
func setupExtensionHost(ctx context.Context, cfg *config.Config) (*host.Host, error) {
	return host.New(ctx, host.Options{
		RuntimeDir:          filepath.Join(cfg.ExecRoot, "extensions"),
		Extensions:          builtinExtensions(cfg),
		Dirs:                extensionDirs(cfg),
		ClientProviders:     clientProviders(),
		DependencyProviders: dependencyProviders(),
		ExtensionConfig:     extensionConfig(cfg),
	})
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
