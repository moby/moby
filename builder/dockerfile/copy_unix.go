//go:build !windows
// +build !windows

package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/containerfs"
	"github.com/docker/docker/pkg/idtools"
)

func fixPermissions(source, destination string, identity idtools.Identity, overrideSkip bool) error {
	var (
		skipChownRoot bool
		err           error
	)
	if !overrideSkip {
		destEndpoint := &copyEndpoint{driver: containerfs.NewLocalDriver(), path: destination}
		skipChownRoot, err = isExistingDirectory(destEndpoint)
		if err != nil {
			return err
		}
	}

	// We Walk on the source rather than on the destination because we don't
	// want to change permissions on things we haven't created or modified.
	return filepath.Walk(source, func(fullpath string, _ os.FileInfo, _ error) error {
		// Do not alter the walk root iff. it existed before, as it doesn't fall under
		// the domain of "things we should chown".
		if skipChownRoot && source == fullpath {
			return nil
		}

		// Path is prefixed by source: substitute with destination instead.
		cleaned, err := filepath.Rel(source, fullpath)
		if err != nil {
			return err
		}

		fullpath = filepath.Join(destination, cleaned)
		return os.Lchown(fullpath, identity.UID, identity.GID)
	})
}

// normalizeDest normalises the destination of a COPY/ADD command in a
// platform semantically consistent way.
func normalizeDest(workingDir, requested string) (string, error) {
	dest := filepath.FromSlash(requested)
	endsInSlash := strings.HasSuffix(dest, string(os.PathSeparator))

	if !path.IsAbs(requested) {
		dest = path.Join("/", filepath.ToSlash(workingDir), dest)
		// Make sure we preserve any trailing slash
		if endsInSlash {
			dest += "/"
		}
	}
	return dest, nil
}

func containsWildcards(name string) bool {
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '\\' {
			i++
		} else if ch == '*' || ch == '?' || ch == '[' {
			return true
		}
	}
	return false
}

func validateCopySourcePath(imageSource *imageMount, origPath string) error {
	return nil
}
