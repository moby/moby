// +build !windows

package dockerfile2llb

func defaultShell() []string {
	return []string{"/bin/sh", "-c"}
}
