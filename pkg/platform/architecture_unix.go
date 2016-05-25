// +build freebsd solaris

package platform

import (
	"os/exec"
	"strings"
)

// runtimeArchitecture get the name of the current architecture (i86pc, sun4v)
func runtimeArchitecture() (string, error) {
	cmd := exec.Command("/usr/bin/uname", "-m")
	machine, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(machine)), nil
}
