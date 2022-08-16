package operatingsystem // import "github.com/docker/docker/pkg/parsers/operatingsystem"

import (
	"errors"

	"github.com/Microsoft/hcsshim/osversion"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// VER_NT_WORKSTATION, see https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-osversioninfoexa
const verNTWorkstation = 0x00000001 // VER_NT_WORKSTATION

// GetOperatingSystem gets the name of the current operating system.
func GetOperatingSystem() (string, error) {
	osversion := windows.RtlGetVersion() // Always succeeds.
	rel := windowsOSRelease{
		IsServer: osversion.ProductType != verNTWorkstation,
		Build:    osversion.BuildNumber,
	}

	// Make a best-effort attempt to retrieve the display version and
	// Update Build Revision by querying undocumented registry values.
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err == nil {
		defer key.Close()
		if ver, err := getFirstStringValue(key,
			"DisplayVersion", /* Windows 20H2 and above */
			"ReleaseId",      /* Windows 2009 and below */
		); err == nil {
			rel.DisplayVersion = ver
		}
		if ubr, _, err := key.GetIntegerValue("UBR"); err == nil {
			rel.UBR = ubr
		}
	}

	return rel.String(), nil
}

func getFirstStringValue(key registry.Key, names ...string) (string, error) {
	for _, n := range names {
		val, _, err := key.GetStringValue(n)
		if err != nil {
			if !errors.Is(err, registry.ErrNotExist) {
				return "", err
			}
			continue
		}
		return val, nil
	}
	return "", registry.ErrNotExist
}

// GetOperatingSystemVersion gets the version of the current operating system, as a string.
func GetOperatingSystemVersion() (string, error) {
	return osversion.Get().ToString(), nil
}

// IsContainerized returns true if we are running inside a container.
// No-op on Windows, always returns false.
func IsContainerized() (bool, error) {
	return false, nil
}

// IsWindowsClient returns true if the SKU is client. It returns false on
// Windows server.
func IsWindowsClient() bool {
	return windows.RtlGetVersion().ProductType == verNTWorkstation
}
