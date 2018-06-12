// +build windows

package dockerfile2llb

func defaultShell() []string {
	return []string{"cmd", "/S", "/C"}
}
