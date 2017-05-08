// +build !windows

package idtools

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/system"
)

func mkdirAs(path string, mode os.FileMode, ownerUID, ownerGID int, mkAll, chownExisting bool) error {
	// make an array containing the original path asked for, plus (for mkAll == true)
	// all path components leading up to the complete path that don't exist before we MkdirAll
	// so that we can chown all of them properly at the end.  If chownExisting is false, we won't
	// chown the full directory path if it exists
	var paths []string
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		paths = []string{path}
	} else if err == nil && chownExisting {
		if err := os.Chown(path, ownerUID, ownerGID); err != nil {
			return err
		}
		// short-circuit--we were called with an existing directory and chown was requested
		return nil
	} else if err == nil {
		// nothing to do; directory path fully exists already and chown was NOT requested
		return nil
	}

	if mkAll {
		// walk back to "/" looking for directories which do not exist
		// and add them to the paths array for chown after creation
		dirPath := path
		for {
			dirPath = filepath.Dir(dirPath)
			if dirPath == "/" {
				break
			}
			if _, err := os.Stat(dirPath); err != nil && os.IsNotExist(err) {
				paths = append(paths, dirPath)
			}
		}
		if err := system.MkdirAll(path, mode); err != nil && !os.IsExist(err) {
			return err
		}
	} else {
		if err := os.Mkdir(path, mode); err != nil && !os.IsExist(err) {
			return err
		}
	}
	// even if it existed, we will chown the requested path + any subpaths that
	// didn't exist when we called MkdirAll
	for _, pathComponent := range paths {
		if err := os.Chown(pathComponent, ownerUID, ownerGID); err != nil {
			return err
		}
	}
	return nil
}

// CanAccess takes a valid (existing) directory and a uid, gid pair and determines
// if that uid, gid pair has access (execute bit) to the directory
func CanAccess(path string, uid, gid int) bool {
	statInfo, err := system.Stat(path)
	if err != nil {
		return false
	}
	fileMode := os.FileMode(statInfo.Mode())
	permBits := fileMode.Perm()
	return accessible(statInfo.UID() == uint32(uid),
		statInfo.GID() == uint32(gid), permBits)
}

func accessible(isOwner, isGroup bool, perms os.FileMode) bool {
	if isOwner && (perms&0100 == 0100) {
		return true
	}
	if isGroup && (perms&0010 == 0010) {
		return true
	}
	if perms&0001 == 0001 {
		return true
	}
	return false
}
