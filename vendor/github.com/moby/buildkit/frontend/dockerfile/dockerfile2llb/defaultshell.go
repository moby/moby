package dockerfile2llb

func defaultShell(os string) []string {
	if os == "windows" {
		return []string{"cmd", "/S", "/C"}
	}
	return []string{"/bin/sh", "-c"}
}
