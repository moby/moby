package operatingsystem // import "github.com/docker/docker/pkg/parsers/operatingsystem"

import (
	"fmt"

	"github.com/yusufpapurcu/wmi"
)

// GetOperatingSystem gets the name of the current operating system.
func GetOperatingSystem() (string, error) {
	type win32OS struct {
		BuildNumber string
		Caption     string
	}

	var dst []win32OS
	if err := wmi.Query("SELECT BuildNumber, Caption FROM Win32_OperatingSystem", &dst); err != nil {
		return "", err
	}
	if len(dst) == 0 || dst[0].BuildNumber == "" || dst[0].Caption == "" {
		// Default return value
		return "Unknown Operating System", nil
	}

	return fmt.Sprintf("%s (Build %s)", dst[0].Caption, dst[0].BuildNumber), nil
}

// GetOperatingSystemVersion gets the version of the current operating system, as a string.
func GetOperatingSystemVersion() (string, error) {
	type win32OS struct {
		Version string
	}

	var dst []win32OS
	if err := wmi.Query("SELECT Version FROM Win32_OperatingSystem", &dst); err != nil {
		return "", err
	}
	if len(dst) == 0 || dst[0].Version == "" {
		// Default return value
		return "Unknown Operating System Version", nil
	}

	return dst[0].Version, nil
}

// IsContainerized returns true if we are running inside a container.
// No-op on Windows, always returns false.
func IsContainerized() (bool, error) {
	return false, nil
}
