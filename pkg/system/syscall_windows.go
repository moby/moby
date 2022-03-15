package system // import "github.com/docker/docker/pkg/system"

import "golang.org/x/sys/windows"

// VER_NT_WORKSTATION, see https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-osversioninfoexa
const verNTWorkstation = 0x00000001 // VER_NT_WORKSTATION

// IsWindowsClient returns true if the SKU is client. It returns false on
// Windows server, or if an error occurred when making the GetVersionExW
// syscall.
func IsWindowsClient() bool {
	ver := windows.RtlGetVersion()
	return ver != nil && ver.ProductType == verNTWorkstation
}
