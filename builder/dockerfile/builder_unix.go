// +build !windows

package dockerfile

func defaultShellForOS(os string) []string {
	return []string{"/bin/sh", "-c"}
}
