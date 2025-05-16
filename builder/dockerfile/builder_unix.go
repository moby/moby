//go:build !windows

package dockerfile // import "github.com/docker/docker/builder/dockerfile"

func defaultShellForOS(_ string) []string {
	return []string{"/bin/sh", "-c"}
}
