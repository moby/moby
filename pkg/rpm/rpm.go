package rpm

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// Version returns package version for the specified package or executable path
func Version(name string) (string, error) {
	options := "-q"
	if filepath.IsAbs(name) {
		options = options + "f"
	}
	rpmPath, err := exec.LookPath("rpm")
	if err != nil {
		return "", err
	}
	out, err := exec.Command(rpmPath, options, name).Output()
	return strings.TrimSpace(string(out)), err
}
