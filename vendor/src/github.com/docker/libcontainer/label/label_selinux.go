// +build selinux,linux

package label

import (
	"fmt"
	"strings"

	"github.com/docker/libcontainer/selinux"
)

func GenLabels(options string) (string, string, error) {
	if !selinux.SelinuxEnabled() {
		return "", "", nil
	}
	var err error
	processLabel, mountLabel := selinux.GetLxcContexts()
	if processLabel != "" {
		var (
			s = strings.Fields(options)
			l = len(s)
		)
		if l > 0 {
			pcon := selinux.NewContext(processLabel)
			for i := 0; i < l; i++ {
				o := strings.Split(s[i], "=")
				pcon[o[0]] = o[1]
			}
			processLabel = pcon.Get()
			mountLabel, err = selinux.CopyLevel(processLabel, mountLabel)
		}
	}
	return processLabel, mountLabel, err
}

func FormatMountLabel(src, mountLabel string) string {
	if mountLabel != "" {
		switch src {
		case "":
			src = fmt.Sprintf("context=%q", mountLabel)
		default:
			src = fmt.Sprintf("%s,context=%q", src, mountLabel)
		}
	}
	return src
}

func SetProcessLabel(processLabel string) error {
	if selinux.SelinuxEnabled() {
		return selinux.Setexeccon(processLabel)
	}
	return nil
}

func GetProcessLabel() (string, error) {
	if selinux.SelinuxEnabled() {
		return selinux.Getexeccon()
	}
	return "", nil
}

func SetFileLabel(path string, fileLabel string) error {
	if selinux.SelinuxEnabled() && fileLabel != "" {
		return selinux.Setfilecon(path, fileLabel)
	}
	return nil
}

// Change the label of path to the filelabel string.  If the relabel string
// is "z", relabel will change the MCS label to s0.  This will allow all
// containers to share the content.  If the relabel string is a "Z" then
// the MCS label should continue to be used.  SELinux will use this field
// to make sure the content can not be shared by other containes.
func Relabel(path string, fileLabel string, relabel string) error {
	if fileLabel == "" {
		return nil
	}
	if relabel == "z" {
		c := selinux.NewContext(fileLabel)
		c["level"] = "s0"
		fileLabel = c.Get()
	}
	return selinux.Chcon(path, fileLabel, true)
}

func GetPidCon(pid int) (string, error) {
	if !selinux.SelinuxEnabled() {
		return "", nil
	}
	return selinux.Getpidcon(pid)
}

func Init() {
	selinux.SelinuxEnabled()
}

func ReserveLabel(label string) error {
	selinux.ReserveLabel(label)
	return nil
}
