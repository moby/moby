//go:build windows
// +build windows

package fs

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

const (
	containerAdministratorSidString = "S-1-5-93-2-1"
)

func fixRootDirectory(p string) string {
	if len(p) == len(`\\?\c:`) {
		if os.IsPathSeparator(p[0]) && os.IsPathSeparator(p[1]) && p[2] == '?' && os.IsPathSeparator(p[3]) && p[5] == ':' {
			return p + `\`
		}
	}
	return p
}

func Utimes(p string, tm *time.Time) error {
	return nil
}

func Chown(p string, old *User, fn Chowner) error {
	if fn == nil {
		return nil
	}
	user, err := fn(old)
	if err != nil {
		return errors.WithStack(err)
	}

	userSIDstring := user.SID
	if userSIDstring == "" {
		userSIDstring = containerAdministratorSidString

	}
	// Copy file ownership and ACL
	// We need SeRestorePrivilege and SeTakeOwnershipPrivilege in order
	// to restore security info on a file, especially if we're trying to
	// apply security info which includes SIDs not necessarily present on
	// the host.
	privileges := []string{winio.SeRestorePrivilege, seTakeOwnershipPrivilege}
	if err := winio.EnableProcessPrivileges(privileges); err != nil {
		return err
	}
	defer winio.DisableProcessPrivileges(privileges)

	sidPtr, err := syscall.UTF16PtrFromString(userSIDstring)
	if err != nil {
		return errors.Wrap(err, "converting to utf16 ptr")
	}
	var userSID *windows.SID
	if err := windows.ConvertStringSidToSid(sidPtr, &userSID); err != nil {
		return errors.Wrap(err, "converting to windows SID")
	}
	var dacl *windows.ACL
	newEntries := []windows.EXPLICIT_ACCESS{
		{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeValue: windows.TrusteeValueFromSID(userSID),
			},
		},
	}
	newAcl, err := windows.ACLFromEntries(newEntries, dacl)
	if err != nil {
		return fmt.Errorf("adding acls: %w", err)
	}

	if err := windows.SetNamedSecurityInfo(
		p, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
		userSID, nil, newAcl, nil); err != nil {

		return err
	}

	return nil
}
