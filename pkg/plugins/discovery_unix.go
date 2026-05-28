//go:build !windows

package plugins

import (
	"os"
	"path/filepath"

	"github.com/moby/moby/v2/pkg/homedir"
)

func rootlessConfigPluginsPath() string {
	if configHome, err := homedir.GetConfigHome(); err != nil {
		return filepath.Join(configHome, "docker/plugins")
	}
	return "/etc/docker/plugins"
}

func rootlessLibPluginsPath() string {
	if libHome, err := homedir.GetLibHome(); err == nil {
		return filepath.Join(libHome, "docker/plugins")
	}
	return "/usr/lib/docker/plugins"
}

// specsPaths is the non-Windows implementation of [SpecsPaths].
func specsPaths() []string {
	// TODO(thaJeztah): switch back to daemon/internal/rootless.RunningWithRootlessKit if this package moves internal to the daemon.
	if os.Getenv("ROOTLESSKIT_STATE_DIR") != "" {
		return []string{rootlessConfigPluginsPath(), rootlessLibPluginsPath()}
	}
	return []string{"/etc/docker/plugins", "/usr/lib/docker/plugins"}
}
