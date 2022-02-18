//go:build !windows
// +build !windows

package plugins // import "github.com/moby/moby/pkg/plugins"

var specsPaths = []string{"/etc/docker/plugins", "/usr/lib/docker/plugins"}
