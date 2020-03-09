package system // import "github.com/docker/docker/pkg/system"

import (
	"syscall"
	"unsafe"

	"github.com/Microsoft/hcsshim/osversion"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

const (
	OWNER_SECURITY_INFORMATION               = windows.OWNER_SECURITY_INFORMATION     // Deprecated: use golang.org/x/sys/windows.OWNER_SECURITY_INFORMATION
	GROUP_SECURITY_INFORMATION               = windows.GROUP_SECURITY_INFORMATION     // Deprecated: use golang.org/x/sys/windows.GROUP_SECURITY_INFORMATION
	DACL_SECURITY_INFORMATION                = windows.DACL_SECURITY_INFORMATION      // Deprecated: use golang.org/x/sys/windows.DACL_SECURITY_INFORMATION
	SACL_SECURITY_INFORMATION                = windows.SACL_SECURITY_INFORMATION      // Deprecated: use golang.org/x/sys/windows.SACL_SECURITY_INFORMATION
	LABEL_SECURITY_INFORMATION               = windows.LABEL_SECURITY_INFORMATION     // Deprecated: use golang.org/x/sys/windows.LABEL_SECURITY_INFORMATION
	ATTRIBUTE_SECURITY_INFORMATION           = windows.ATTRIBUTE_SECURITY_INFORMATION // Deprecated: use golang.org/x/sys/windows.ATTRIBUTE_SECURITY_INFORMATION
	SCOPE_SECURITY_INFORMATION               = windows.SCOPE_SECURITY_INFORMATION     // Deprecated: use golang.org/x/sys/windows.SCOPE_SECURITY_INFORMATION
	PROCESS_TRUST_LABEL_SECURITY_INFORMATION = 0x00000080
	ACCESS_FILTER_SECURITY_INFORMATION       = 0x00000100
	BACKUP_SECURITY_INFORMATION              = windows.BACKUP_SECURITY_INFORMATION           // Deprecated: use golang.org/x/sys/windows.BACKUP_SECURITY_INFORMATION
	PROTECTED_DACL_SECURITY_INFORMATION      = windows.PROTECTED_DACL_SECURITY_INFORMATION   // Deprecated: use golang.org/x/sys/windows.PROTECTED_DACL_SECURITY_INFORMATION
	PROTECTED_SACL_SECURITY_INFORMATION      = windows.PROTECTED_SACL_SECURITY_INFORMATION   // Deprecated: use golang.org/x/sys/windows.PROTECTED_SACL_SECURITY_INFORMATION
	UNPROTECTED_DACL_SECURITY_INFORMATION    = windows.UNPROTECTED_DACL_SECURITY_INFORMATION // Deprecated: use golang.org/x/sys/windows.UNPROTECTED_DACL_SECURITY_INFORMATION
	UNPROTECTED_SACL_SECURITY_INFORMATION    = windows.UNPROTECTED_SACL_SECURITY_INFORMATION // Deprecated: use golang.org/x/sys/windows.UNPROTECTED_SACL_SECURITY_INFORMATION
)

const (
	SE_UNKNOWN_OBJECT_TYPE     = windows.SE_UNKNOWN_OBJECT_TYPE     // Deprecated: use golang.org/x/sys/windows.SE_UNKNOWN_OBJECT_TYPE
	SE_FILE_OBJECT             = windows.SE_FILE_OBJECT             // Deprecated: use golang.org/x/sys/windows.SE_FILE_OBJECT
	SE_SERVICE                 = windows.SE_SERVICE                 // Deprecated: use golang.org/x/sys/windows.SE_SERVICE
	SE_PRINTER                 = windows.SE_PRINTER                 // Deprecated: use golang.org/x/sys/windows.SE_PRINTER
	SE_REGISTRY_KEY            = windows.SE_REGISTRY_KEY            // Deprecated: use golang.org/x/sys/windows.SE_REGISTRY_KEY
	SE_LMSHARE                 = windows.SE_LMSHARE                 // Deprecated: use golang.org/x/sys/windows.SE_LMSHARE
	SE_KERNEL_OBJECT           = windows.SE_KERNEL_OBJECT           // Deprecated: use golang.org/x/sys/windows.SE_KERNEL_OBJECT
	SE_WINDOW_OBJECT           = windows.SE_WINDOW_OBJECT           // Deprecated: use golang.org/x/sys/windows.SE_WINDOW_OBJECT
	SE_DS_OBJECT               = windows.SE_DS_OBJECT               // Deprecated: use golang.org/x/sys/windows.SE_DS_OBJECT
	SE_DS_OBJECT_ALL           = windows.SE_DS_OBJECT_ALL           // Deprecated: use golang.org/x/sys/windows.SE_DS_OBJECT_ALL
	SE_PROVIDER_DEFINED_OBJECT = windows.SE_PROVIDER_DEFINED_OBJECT // Deprecated: use golang.org/x/sys/windows.SE_PROVIDER_DEFINED_OBJECT
	SE_WMIGUID_OBJECT          = windows.SE_WMIGUID_OBJECT          // Deprecated: use golang.org/x/sys/windows.SE_WMIGUID_OBJECT
	SE_REGISTRY_WOW64_32KEY    = windows.SE_REGISTRY_WOW64_32KEY    // Deprecated: use golang.org/x/sys/windows.SE_REGISTRY_WOW64_32KEY
)

const (
	SeTakeOwnershipPrivilege = "SeTakeOwnershipPrivilege"
)

const (
	ContainerAdministratorSidString = "S-1-5-93-2-1"
	ContainerUserSidString          = "S-1-5-93-2-2"
)

var (
	ntuserApiset                  = windows.NewLazyDLL("ext-ms-win-ntuser-window-l1-1-0")
	modadvapi32                   = windows.NewLazySystemDLL("advapi32.dll")
	procGetVersionExW             = modkernel32.NewProc("GetVersionExW")
	procSetNamedSecurityInfo      = modadvapi32.NewProc("SetNamedSecurityInfoW")
	procGetSecurityDescriptorDacl = modadvapi32.NewProc("GetSecurityDescriptorDacl")
)

// OSVersion is a wrapper for Windows version information
// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724439(v=vs.85).aspx
type OSVersion = osversion.OSVersion

// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724833(v=vs.85).aspx
// TODO: use golang.org/x/sys/windows.OsVersionInfoEx (needs OSVersionInfoSize to be exported)
type osVersionInfoEx struct {
	OSVersionInfoSize uint32
	MajorVersion      uint32
	MinorVersion      uint32
	BuildNumber       uint32
	PlatformID        uint32
	CSDVersion        [128]uint16
	ServicePackMajor  uint16
	ServicePackMinor  uint16
	SuiteMask         uint16
	ProductType       byte
	Reserve           byte
}

// GetOSVersion gets the operating system version on Windows. Note that
// dockerd.exe must be manifested to get the correct version information.
// Deprecated: use github.com/Microsoft/hcsshim/osversion.Get() instead
func GetOSVersion() OSVersion {
	return osversion.Get()
}

// IsWindowsClient returns true if the SKU is client
func IsWindowsClient() bool {
	osviex := &osVersionInfoEx{OSVersionInfoSize: 284}
	r1, _, err := procGetVersionExW.Call(uintptr(unsafe.Pointer(osviex)))
	if r1 == 0 {
		logrus.Warnf("GetVersionExW failed - assuming server SKU: %v", err)
		return false
	}
	const verNTWorkstation = 0x00000001
	return osviex.ProductType == verNTWorkstation
}

// Unmount is a platform-specific helper function to call
// the unmount syscall. Not supported on Windows
func Unmount(_ string) error {
	return nil
}

// HasWin32KSupport determines whether containers that depend on win32k can
// run on this machine. Win32k is the driver used to implement windowing.
func HasWin32KSupport() bool {
	// For now, check for ntuser API support on the host. In the future, a host
	// may support win32k in containers even if the host does not support ntuser
	// APIs.
	return ntuserApiset.Load() == nil
}

func SetNamedSecurityInfo(objectName *uint16, objectType uint32, securityInformation uint32, sidOwner *windows.SID, sidGroup *windows.SID, dacl *byte, sacl *byte) (result error) {
	r0, _, _ := syscall.Syscall9(procSetNamedSecurityInfo.Addr(), 7, uintptr(unsafe.Pointer(objectName)), uintptr(objectType), uintptr(securityInformation), uintptr(unsafe.Pointer(sidOwner)), uintptr(unsafe.Pointer(sidGroup)), uintptr(unsafe.Pointer(dacl)), uintptr(unsafe.Pointer(sacl)), 0, 0)
	if r0 != 0 {
		result = syscall.Errno(r0)
	}
	return
}

func GetSecurityDescriptorDacl(securityDescriptor *byte, daclPresent *uint32, dacl **byte, daclDefaulted *uint32) (result error) {
	r1, _, e1 := syscall.Syscall6(procGetSecurityDescriptorDacl.Addr(), 4, uintptr(unsafe.Pointer(securityDescriptor)), uintptr(unsafe.Pointer(daclPresent)), uintptr(unsafe.Pointer(dacl)), uintptr(unsafe.Pointer(daclDefaulted)), 0, 0)
	if r1 == 0 {
		if e1 != 0 {
			result = e1
		} else {
			result = syscall.EINVAL
		}
	}
	return
}
