package archive

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// ApplyLayer parses a diff in the standard layer format from `layer`, and
// applies it to the directory `dest`.
func ApplyLayer(dest string, layer Archive) error {
	// Poor man's diff applyer in 2 steps:

	// Step 1: untar everything in place
	if err := Untar(layer, dest, nil); err != nil {
		return err
	}

	modifiedDirs := make(map[string]*syscall.Stat_t)
	addDir := func(file string) {
		d := filepath.Dir(file)
		if _, exists := modifiedDirs[d]; !exists {
			if s, err := os.Lstat(d); err == nil {
				if sys := s.Sys(); sys != nil {
					if stat, ok := sys.(*syscall.Stat_t); ok {
						modifiedDirs[d] = stat
					}
				}
			}
		}
	}

	// Step 2: walk for whiteouts and apply them, removing them in the process
	err := filepath.Walk(dest, func(fullPath string, f os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				// This happens in the case of whiteouts in parent dir removing a directory
				// We just ignore it
				return filepath.SkipDir
			}
			return err
		}

		// Rebase path
		path, err := filepath.Rel(dest, fullPath)
		if err != nil {
			return err
		}
		path = filepath.Join("/", path)

		// Skip AUFS metadata
		if matched, err := filepath.Match("/.wh..wh.*", path); err != nil {
			return err
		} else if matched {
			addDir(fullPath)
			if err := os.RemoveAll(fullPath); err != nil {
				return err
			}
		}

		filename := filepath.Base(path)
		if strings.HasPrefix(filename, ".wh.") {
			rmTargetName := filename[len(".wh."):]
			rmTargetPath := filepath.Join(filepath.Dir(fullPath), rmTargetName)

			// Remove the file targeted by the whiteout
			addDir(rmTargetPath)
			if err := os.RemoveAll(rmTargetPath); err != nil {
				return err
			}
			// Remove the whiteout itself
			addDir(fullPath)
			if err := os.RemoveAll(fullPath); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for k, v := range modifiedDirs {
		lastAccess := getLastAccess(v)
		lastModification := getLastModification(v)
		aTime := time.Unix(lastAccess.Unix())
		mTime := time.Unix(lastModification.Unix())

		if err := os.Chtimes(k, aTime, mTime); err != nil {
			return err
		}
	}

	return nil
}
