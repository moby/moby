//+build !selinux

package label

func GenLabels(options string) (string, string, error) {
	return "", "", nil
}

func FormatMountLabel(src string, MountLabel string) string {
	return src
}

func SetProcessLabel(label string) error {
	return nil
}

func SetFileLabel(path string, label string) error {
	return nil
}

func GetPidCon(pid int) (string, error) {
	return "", nil
}
