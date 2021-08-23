//go:build darwin
// +build darwin

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
	var release string
	data := strings.Split(osName, "\n")
	for _, line := range data {
		if !strings.Contains(line, "Kernel Version") {
			continue
		}
		// It has the format like '      Kernel Version: Darwin 14.5.0'
		content := strings.SplitN(line, ":", 2)
		if len(content) != 2 {
			return "", fmt.Errorf("Kernel Version is invalid")
		}

		prettyNames := strings.SplitN(strings.TrimSpace(content[1]), " ", 2)

		if len(prettyNames) != 2 {
			return "", fmt.Errorf("Kernel Version needs to be 'Darwin x.x.x' ")
		}
		release = prettyNames[1]
	}

	return release, nil
}

func getSPSoftwareDataType() (string, error) {
	cmd := exec.Command("system_profiler", "SPSoftwareDataType")
	osName, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(osName), nil
}
