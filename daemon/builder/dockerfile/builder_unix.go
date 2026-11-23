//go:build !windows

package dockerfile

func defaultShellForOS(_ string) []string {
	return []string{"/bin/sh", "-c"}
}
