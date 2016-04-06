package platform

import (
	"os/exec"
)

// runtimeArchitecture get the name of the current architecture (x86, x86_64, …)
func runtimeArchitecture() (string, error) {
	cmd := exec.Command("uname", "-m")
	machine, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(machine), nil
}
