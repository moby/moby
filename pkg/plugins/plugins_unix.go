//go:build !windows

package plugins // import "github.com/docker/docker/pkg/plugins"

// ScopedPath returns the path scoped to the plugin's rootfs.
// For v1 plugins, this always returns the path unchanged as v1 plugins run directly on the host.
func (p *Plugin) ScopedPath(s string) string {
	return s
}
