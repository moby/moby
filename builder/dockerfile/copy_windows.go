package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	winio "github.com/Microsoft/go-winio"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

var pathDenyList = map[string]bool{
	"c:\\":        true,
	"c:\\windows": true,
}

func init() {
	reexec.Register("windows-fix-permissions", fixPermissionsReexec)
}

func fixPermissions(source, destination string, identity idtools.Identity, _ bool) error {
	if identity.SID == "" {
		return nil
	}

	cmd := reexec.Command("windows-fix-permissions", source, destination, identity.SID)
	output, err := cmd.CombinedOutput()

	return errors.Wrapf(err, "failed to exec windows-fix-permissions: %s", output)
}

func fixPermissionsReexec() {
	err := fixPermissionsWindows(os.Args[1], os.Args[2], os.Args[3])
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func fixPermissionsWindows(source, destination, SID string) error {
	privileges := []string{winio.SeRestorePrivilege, idtools.SeTakeOwnershipPrivilege}

	err := winio.EnableProcessPrivileges(privileges)
	if err != nil {
		return err
	}

	defer winio.DisableProcessPrivileges(privileges)

	sid, err := windows.StringToSid(SID)
	if err != nil {
		return err
	}

	// Owners on *nix have read/write/delete/read control and write DAC.
	// Add an ACE that grants this to the user/group specified with the
	// chown option. Currently Windows is not honoring the owner change,
	// however, they are aware of this and it should be fixed at some
	// point.

	sddlString := system.SddlAdministratorsLocalSystem
	sddlString += "(A;OICI;GRGWGXRCWDSD;;;" + SID + ")"

	securityDescriptor, err := windows.SecurityDescriptorFromString(sddlString)
	if err != nil {
		return err
	}

	dacl, _, err := securityDescriptor.DACL()
	if err != nil {
		return err
	}

	return windows.SetNamedSecurityInfo(destination, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION, sid, nil, dacl, nil)
}

// normalizeDest normalises the destination of a COPY/ADD command in a
// platform semantically consistent way.
func normalizeDest(workingDir, requested string) (string, error) {
	dest := filepath.FromSlash(requested)
	endsInSlash := strings.HasSuffix(dest, string(os.PathSeparator))

	// We are guaranteed that the working directory is already consistent,
	// However, Windows also has, for now, the limitation that ADD/COPY can
	// only be done to the system drive, not any drives that might be present
	// as a result of a bind mount.
	//
	// So... if the path requested is Linux-style absolute (/foo or \\foo),
	// we assume it is the system drive. If it is a Windows-style absolute
	// (DRIVE:\\foo), error if DRIVE is not C. And finally, ensure we
	// strip any configured working directories drive letter so that it
	// can be subsequently legitimately converted to a Windows volume-style
	// pathname.

	// Not a typo - filepath.IsAbs, not system.IsAbs on this next check as
	// we only want to validate where the DriveColon part has been supplied.
	if filepath.IsAbs(dest) {
		if strings.ToUpper(string(dest[0])) != "C" {
			return "", fmt.Errorf("Windows does not support destinations not on the system drive (C:)")
		}
		dest = dest[2:] // Strip the drive letter
	}

	// Cannot handle relative where WorkingDir is not the system drive.
	if len(workingDir) > 0 {
		if ((len(workingDir) > 1) && !system.IsAbs(workingDir[2:])) || (len(workingDir) == 1) {
			return "", fmt.Errorf("Current WorkingDir %s is not platform consistent", workingDir)
		}
		if !system.IsAbs(dest) {
			if string(workingDir[0]) != "C" {
				return "", fmt.Errorf("Windows does not support relative paths when WORKDIR is not the system drive")
			}
			dest = filepath.Join(string(os.PathSeparator), workingDir[2:], dest)
			// Make sure we preserve any trailing slash
			if endsInSlash {
				dest += string(os.PathSeparator)
			}
		}
	}
	return dest, nil
}

func containsWildcards(name string) bool {
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '*' || ch == '?' || ch == '[' {
			return true
		}
	}
	return false
}

func validateCopySourcePath(imageSource *imageMount, origPath string) error {
	if imageSource == nil {
		return nil
	}
	origPath = filepath.FromSlash(origPath)
	p := strings.ToLower(filepath.Clean(origPath))
	if !filepath.IsAbs(p) {
		if filepath.VolumeName(p) != "" {
			if p[len(p)-2:] == ":." { // case where clean returns weird c:. paths
				p = p[:len(p)-1]
			}
			p += `\`
		} else {
			p = filepath.Join(`c:\`, p)
		}
	}
	if _, ok := pathDenyList[p]; ok {
		return errors.New(`copy from c:\ or c:\windows is not allowed on windows`)
	}
	return nil
}
