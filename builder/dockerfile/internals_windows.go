package dockerfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/system"
)

// normaliseDest normalises the destination of a COPY/ADD command in a
// platform semantically consistent way.
func normaliseDest(cmdName, workingDir, requested string) (string, error) {
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
			return "", fmt.Errorf("Windows does not support %s with a destinations not on the system drive (C:)", cmdName)
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
				return "", fmt.Errorf("Windows does not support %s with relative paths when WORKDIR is not the system drive", cmdName)
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
