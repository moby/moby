package dockerfile

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/idtools"
)

func fixPermissions(source, destination string, rootIDs idtools.IDPair, overrideSkip bool) error {
	// chown is not supported on Windows
	return nil
}

func validateCopySourcePath(imageSource *imageMount, origPath, platform string) error {
	// validate windows paths from other images + LCOW
	if imageSource == nil || platform != "windows" {
		return nil
	}

	origPath = filepath.FromSlash(origPath)
	p := strings.ToLower(filepath.Clean(origPath))
	if !filepath.IsAbs(p) {
		if filepath.VolumeName(p) != "" {
			if p[len(p)-2:] == ":." { // case where clean returns weird c:. paths
				p = p[:len(p)-1]
			}
			p += "\\"
		} else {
			p = filepath.Join("c:\\", p)
		}
	}
	if _, blacklisted := pathBlacklist[p]; blacklisted {
		return errors.New("copy from c:\\ or c:\\windows is not allowed on windows")
	}
	return nil
}
