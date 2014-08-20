// +build !selinux !linux

package label

// InitLabels returns the process label and file labels to be used within
// the container.  A list of options can be passed into this function to alter
// the labels.
func InitLabels(options []string) (string, string, error) {
	return "", "", nil
}

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

func Relabel(path string, fileLabel string, relabel string) error {
	return nil
}

func GetPidLabel(pid int) (string, error) {
	return "", nil
}

func Init() {
}

func ReserveLabel(label string) error {
	return nil
}
