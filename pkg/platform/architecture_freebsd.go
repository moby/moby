package platform

import (
	"os/exec"
)

// GetRuntimeArchitecture get the name of the current architecture (x86, x86_64, …)
func GetRuntimeArchitecture() (string, error) {
	cmd := exec.Command("uname", "-m")
	machine, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(machine), nil
}
