//+build selinux

package label

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/selinux"
	"strings"
)

func GenLabels(options string) (string, string, error) {
	processLabel, mountLabel := selinux.GetLxcContexts()
	var err error
	if processLabel == "" { // SELinux is disabled
		return "", "", err
	}
	s := strings.Fields(options)
	l := len(s)
	if l > 0 {
		pcon := selinux.NewContext(processLabel)
		for i := 0; i < l; i++ {
			o := strings.Split(s[i], "=")
			if o[0] == "user" {
				pcon.SetUser(o[1])
			}
			if o[0] == "role" {
				pcon.SetRole(o[1])
			}
			if o[0] == "type" {
				pcon.SetType(o[1])
			}
			if o[0] == "level" {
				pcon.SetLevel(o[1])
			}
		}
		processLabel = pcon.Get()
		mountLabel, err = selinux.CopyLevel(processLabel, mountLabel)
	}
	return processLabel, mountLabel, err
}

func FormatMountLabel(src string, MountLabel string) string {
	var mountLabel string
	if src != "" {
		mountLabel = src
		if MountLabel != "" {
			mountLabel = fmt.Sprintf("%s,context=\"%s\"", mountLabel, MountLabel)
		}
	} else {
		if MountLabel != "" {
			mountLabel = fmt.Sprintf("context=\"%s\"", mountLabel, MountLabel)
		}
	}
	return mountLabel
}

func SetProcessLabel(processLabel string) error {
	if selinux.SelinuxEnabled() {
		return selinux.Setexeccon(processLabel)
	}
	return nil
}

func SetFileLabel(path string, fileLabel string) error {
	if selinux.SelinuxEnabled() && fileLabel != "" {
		return selinux.Setfilecon(path, fileLabel)
	}
	return nil
}

func GetPidCon(pid int) (string, error) {
	return selinux.Getpidcon(pid)
}
