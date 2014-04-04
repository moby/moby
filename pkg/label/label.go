// +build !selinux !linux

package label

func GenLabels(options string) (string, string, error) {
	return "", "", nil
}

func FormatMountLabel(src string, mountLabel string) string {
	return src
}

func SetProcessLabel(processLabel string) error {
	return nil
}

func SetFileLabel(path string, fileLabel string) error {
	return nil
}

func GetPidCon(pid int) (string, error) {
	return "", nil
}

func Init() {
}
