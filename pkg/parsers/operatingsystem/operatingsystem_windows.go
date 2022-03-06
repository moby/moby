package operatingsystem // import "github.com/docker/docker/pkg/parsers/operatingsystem"

import (
	"fmt"
	"unsafe"

	"github.com/Microsoft/hcsshim/osversion"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	libWinbrand          = windows.NewLazySystemDLL("winbrand.dll")
	libKernel32          = windows.NewLazySystemDLL("kernel32.dll")
	brandingFormatString = libWinbrand.NewProc("BrandingFormatString")
	globalFree           = libKernel32.NewProc("GlobalFree")
)

// GetOperatingSystem gets the name of the current operating system.
func GetOperatingSystem() (string, error) {
	os, err := callBrandingFormatString()
	if err != nil {
		// Default return value
		return "Unknown Operating System", nil
	}

	version, err := withCurrentVersionRegistryKey(func(key registry.Key) (version string, err error) {
		version, _, err = key.GetStringValue("DisplayVersion")
		if err != nil || version == "" {
			// Fallback.
			version, _, err = key.GetStringValue("ReleaseId")
			if err != nil {
				return "", err
			}
		}
		buildNumber, _, err := key.GetStringValue("CurrentBuildNumber")
		if err != nil {
			return "", err
		}
		ubr, _, err := key.GetIntegerValue("UBR")
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Version %s (OS Build %s.%d)", version, buildNumber, ubr), nil
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s %s", os, version), nil
}

func callBrandingFormatString() (string, error) {
	if err := brandingFormatString.Find(); err != nil {
		return "", err
	}

	// BrandingFormatString("%WINDOWS_LONG%") returns the OS full name. The 'winver' program also uses this API.
	arg, err := windows.UTF16PtrFromString("%WINDOWS_LONG%")
	if err != nil {
		return "", err
	}

	// The returned error is always non-nil, constructed from the result of GetLastError.
	// Callers must inspect the primary return value to decide whether an error occurred
	// (according to the semantics of the specific function being called) before consulting
	// the error.
	r1, _, err := brandingFormatString.Call(uintptr(unsafe.Pointer(arg)))
	brand := (*uint16)(unsafe.Pointer(r1))
	if brand == nil {
		return "", err
	}
	defer callGlobalFree(r1)

	return windows.UTF16PtrToString(brand), nil
}

func callGlobalFree(v uintptr) {
	// Just in case.
	if err := globalFree.Find(); err != nil {
		return
	}
	globalFree.Call(v)
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
	return osversion.Get().ToString(), nil
}

// IsContainerized returns true if we are running inside a container.
// No-op on Windows, always returns false.
func IsContainerized() (bool, error) {
	return false, nil
}
