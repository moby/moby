//go:build darwin

// Package kernel provides helper function to get, parse and compare kernel
// versions for different platforms.
package kernel // import "github.com/docker/docker/pkg/parsers/kernel"

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetKernelVersion gets the current kernel version.
func GetKernelVersion() (*VersionInfo, error) {
	osName, err := getSPSoftwareDataType()
	if err != nil {
		return nil, err
	}
	release, err := getRelease(osName)
	if err != nil {
		return nil, err
	}
	return ParseRelease(release)
}

// getRelease uses `system_profiler SPSoftwareDataType` to get OSX kernel version
func getRelease(osName string) (string, error) {
	for _, line := range strings.Split(osName, "\n") {
		if !strings.Contains(line, "Kernel Version") {
			continue
		}
		// It has the format like '      Kernel Version: Darwin 14.5.0'
		_, ver, ok := strings.Cut(line, ":")
		if !ok {
			return "", fmt.Errorf("kernel Version is invalid")
		}

		_, release, ok := strings.Cut(strings.TrimSpace(ver), " ")
		if !ok {
			return "", fmt.Errorf("kernel version needs to be 'Darwin x.x.x'")
		}
		return release, nil
	}

	return "", nil
}

func getSPSoftwareDataType() (string, error) {
	cmd := exec.Command("system_profiler", "SPSoftwareDataType")
	osName, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(osName), nil
}
