//go:build windows
// +build windows

package security

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

type (
	accessMask          uint32
	accessMode          uint32
	desiredAccess       uint32
	inheritMode         uint32
	objectType          uint32
	shareMode           uint32
	securityInformation uint32
	trusteeForm         uint32
	trusteeType         uint32
)

type explicitAccess struct {
	accessPermissions accessMask
	accessMode        accessMode
	inheritance       inheritMode
	trustee           trustee
}

type trustee struct {
	multipleTrustee          *trustee
	multipleTrusteeOperation int32
	trusteeForm              trusteeForm
	trusteeType              trusteeType
	name                     uintptr
}

const (
	AccessMaskNone    accessMask = 0
	AccessMaskRead    accessMask = 1 << 31 // GENERIC_READ
	AccessMaskWrite   accessMask = 1 << 30 // GENERIC_WRITE
	AccessMaskExecute accessMask = 1 << 29 // GENERIC_EXECUTE
	AccessMaskAll     accessMask = 1 << 28 // GENERIC_ALL

	accessMaskDesiredPermission = AccessMaskRead

	accessModeGrant accessMode = 1

	desiredAccessReadControl desiredAccess = 0x20000
	desiredAccessWriteDac    desiredAccess = 0x40000

	gvmga = "GrantVmGroupAccess:"

	inheritModeNoInheritance                  inheritMode = 0x0
	inheritModeSubContainersAndObjectsInherit inheritMode = 0x3

	objectTypeFileObject objectType = 0x1

	securityInformationDACL securityInformation = 0x4

	shareModeRead  shareMode = 0x1
	shareModeWrite shareMode = 0x2

	//nolint:stylecheck // ST1003
	sidVmGroup = "S-1-5-83-0"

	trusteeFormIsSid trusteeForm = 0

	trusteeTypeWellKnownGroup trusteeType = 5
)

// GrantVmGroupAccess sets the DACL for a specified file or directory to
// include Grant ACE entries for the VM Group SID. This is a golang re-
// implementation of the same function in vmcompute, just not exported in
// RS5. Which kind of sucks. Sucks a lot :/
func GrantVmGroupAccess(name string) error { //nolint:stylecheck // ST1003
	return GrantVmGroupAccessWithMask(name, accessMaskDesiredPermission)
}

// GrantVmGroupAccessWithMask sets the desired DACL for a specified file or
// directory.
func GrantVmGroupAccessWithMask(name string, access accessMask) error { //nolint:stylecheck // ST1003
	if access == 0 || access<<4 != 0 {
		return fmt.Errorf("invalid access mask: 0x%08x", access)
	}
	// Stat (to determine if `name` is a directory).
	s, err := os.Stat(name)
	if err != nil {
		return fmt.Errorf("%s os.Stat %s: %w", gvmga, name, err)
	}

	// Get a handle to the file/directory. Must defer Close on success.
	fd, err := createFile(name, s.IsDir())
	if err != nil {
		return err // Already wrapped
	}
	defer func() {
		_ = syscall.CloseHandle(fd)
	}()

	// Get the current DACL and Security Descriptor. Must defer LocalFree on success.
	ot := objectTypeFileObject
	si := securityInformationDACL
	sd := uintptr(0)
	origDACL := uintptr(0)
	if err := getSecurityInfo(fd, uint32(ot), uint32(si), nil, nil, &origDACL, nil, &sd); err != nil {
		return fmt.Errorf("%s GetSecurityInfo %s: %w", gvmga, name, err)
	}
	defer func() {
		_, _ = syscall.LocalFree((syscall.Handle)(unsafe.Pointer(sd)))
	}()

	// Generate a new DACL which is the current DACL with the required ACEs added.
	// Must defer LocalFree on success.
	newDACL, err := generateDACLWithAcesAdded(name, s.IsDir(), access, origDACL)
	if err != nil {
		return err // Already wrapped
	}
	defer func() {
		_, _ = syscall.LocalFree((syscall.Handle)(unsafe.Pointer(newDACL)))
	}()

	// And finally use SetSecurityInfo to apply the updated DACL.
	if err := setSecurityInfo(fd, uint32(ot), uint32(si), uintptr(0), uintptr(0), newDACL, uintptr(0)); err != nil {
		return fmt.Errorf("%s SetSecurityInfo %s: %w", gvmga, name, err)
	}

	return nil
}

// createFile is a helper function to call [Nt]CreateFile to get a handle to
// the file or directory.
func createFile(name string, isDir bool) (syscall.Handle, error) {
	namep, err := syscall.UTF16FromString(name)
	if err != nil {
		return 0, fmt.Errorf("syscall.UTF16FromString %s: %w", name, err)
	}
	da := uint32(desiredAccessReadControl | desiredAccessWriteDac)
	sm := uint32(shareModeRead | shareModeWrite)
	fa := uint32(syscall.FILE_ATTRIBUTE_NORMAL)
	if isDir {
		fa = uint32(fa | syscall.FILE_FLAG_BACKUP_SEMANTICS)
	}
	fd, err := syscall.CreateFile(&namep[0], da, sm, nil, syscall.OPEN_EXISTING, fa, 0)
	if err != nil {
		return 0, fmt.Errorf("%s syscall.CreateFile %s: %w", gvmga, name, err)
	}
	return fd, nil
}

// generateDACLWithAcesAdded generates a new DACL with the two needed ACEs added.
// The caller is responsible for LocalFree of the returned DACL on success.
func generateDACLWithAcesAdded(name string, isDir bool, desiredAccess accessMask, origDACL uintptr) (uintptr, error) {
	// Generate pointers to the SIDs based on the string SIDs
	sid, err := syscall.StringToSid(sidVmGroup)
	if err != nil {
		return 0, fmt.Errorf("%s syscall.StringToSid %s %s: %w", gvmga, name, sidVmGroup, err)
	}

	inheritance := inheritModeNoInheritance
	if isDir {
		inheritance = inheritModeSubContainersAndObjectsInherit
	}

	eaArray := []explicitAccess{
		{
			accessPermissions: desiredAccess,
			accessMode:        accessModeGrant,
			inheritance:       inheritance,
			trustee: trustee{
				trusteeForm: trusteeFormIsSid,
				trusteeType: trusteeTypeWellKnownGroup,
				name:        uintptr(unsafe.Pointer(sid)),
			},
		},
	}

	modifiedDACL := uintptr(0)
	if err := setEntriesInAcl(uintptr(uint32(1)), uintptr(unsafe.Pointer(&eaArray[0])), origDACL, &modifiedDACL); err != nil {
		return 0, fmt.Errorf("%s SetEntriesInAcl %s: %w", gvmga, name, err)
	}

	return modifiedDACL, nil
}
