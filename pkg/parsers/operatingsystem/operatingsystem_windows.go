package operatingsystem // import "github.com/docker/docker/pkg/parsers/operatingsystem"

import (
	"fmt"

	"github.com/docker/docker/pkg/system"
	"golang.org/x/sys/windows/registry"
)

// GetOperatingSystem gets the name of the current operating system.
func GetOperatingSystem() (string, error) {
	os, err := withCurrentVersionRegistryKey(func(key registry.Key) (os string, err error) {
		if os, _, err = key.GetStringValue("ProductName"); err != nil {
			return "", err
		}

		releaseId, _, err := key.GetStringValue("ReleaseId")
		if err != nil {
			return
		}
		os = fmt.Sprintf("%s Version %s", os, releaseId)

		buildNumber, _, err := key.GetStringValue("CurrentBuildNumber")
		if err != nil {
			return
		}
		ubr, _, err := key.GetIntegerValue("UBR")
		if err != nil {
			return
		}
		os = fmt.Sprintf("%s (OS Build %s.%d)", os, buildNumber, ubr)

		return
	})

	if os == "" {
		// Default return value
		os = "Unknown Operating System"
	}

	return os, err
}

func withCurrentVersionRegistryKey(f func(registry.Key) (string, error)) (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()
	return f(key)
}

// GetOperatingSystemVersion gets the version of the current operating system, as a string.
func GetOperatingSystemVersion() (string, error) {
	version := system.GetOSVersion()
	return fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.Build), nil
}

// IsContainerized returns true if we are running inside a container.
// No-op on Windows, always returns false.
func IsContainerized() (bool, error) {
	return false, nil
}
