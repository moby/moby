//go:build !windows
// +build !windows

package dockerfile // import "github.com/moby/moby/builder/dockerfile"

func defaultShellForOS(os string) []string {
	return []string{"/bin/sh", "-c"}
}
