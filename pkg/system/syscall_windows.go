package system // import "github.com/docker/docker/pkg/system"

import "golang.org/x/sys/windows"

const (
	// Deprecated: use github.com/docker/pkg/idtools.SeTakeOwnershipPrivilege
	SeTakeOwnershipPrivilege = "SeTakeOwnershipPrivilege"
	// Deprecated: use github.com/docker/pkg/idtools.ContainerAdministratorSidString
	ContainerAdministratorSidString = "S-1-5-93-2-1"
	// Deprecated: use github.com/docker/pkg/idtools.ContainerUserSidString
	ContainerUserSidString = "S-1-5-93-2-2"
)

// VER_NT_WORKSTATION, see https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-osversioninfoexa
const verNTWorkstation = 0x00000001 // VER_NT_WORKSTATION

// IsWindowsClient returns true if the SKU is client. It returns false on
// Windows server, or if an error occurred when making the GetVersionExW
// syscall.
func IsWindowsClient() bool {
	ver := windows.RtlGetVersion()
	return ver != nil && ver.ProductType == verNTWorkstation
}
