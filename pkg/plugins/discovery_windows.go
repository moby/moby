package plugins // import "github.com/docker/docker/pkg/plugins"

import (
	"os"
	"path/filepath"
)

var globalSpecsPaths = []string{filepath.Join(os.Getenv("programdata"), "docker", "plugins")}

// SpecsPaths returns
// { "%programdata%\docker\plugins" } on Windows,
// { "/etc/docker/plugins", "/usr/lib/docker/plugins" } on Unix in non-rootless mode,
// { "$XDG_CONFIG_HOME/docker/plugins", "$HOME/.local/lib/docker/plugins" } on Unix in rootless mode
// with fallback to the corresponding path in non-rootless mode if $XDG_CONFIG_HOME or $HOME is not set.
func SpecsPaths() []string {
	return globalSpecsPaths
}
