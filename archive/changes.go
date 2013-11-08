package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type ChangeType int

const (
	ChangeModify = iota
	ChangeAdd
	ChangeDelete
)

type Change struct {
	Path string
	Kind ChangeType
}

func (change *Change) String() string {
	var kind string
	switch change.Kind {
	case ChangeModify:
		kind = "C"
	case ChangeAdd:
		kind = "A"
	case ChangeDelete:
		kind = "D"
	}
	return fmt.Sprintf("%s %s", kind, change.Path)
}

func Changes(layers []string, rw string) ([]Change, error) {
	var changes []Change
	err := filepath.Walk(rw, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Rebase path
		path, err = filepath.Rel(rw, path)
		if err != nil {
			return err
		}
		path = filepath.Join("/", path)

		// Skip root
		if path == "/" {
			return nil
		}

		// Skip AUFS metadata
		if matched, err := filepath.Match("/.wh..wh.*", path); err != nil || matched {
			return err
		}

		change := Change{
			Path: path,
		}

		// Find out what kind of modification happened
		file := filepath.Base(path)
		// If there is a whiteout, then the file was removed
		if strings.HasPrefix(file, ".wh.") {
			originalFile := file[len(".wh."):]
			change.Path = filepath.Join(filepath.Dir(path), originalFile)
			change.Kind = ChangeDelete
		} else {
			// Otherwise, the file was added
			change.Kind = ChangeAdd

			// ...Unless it already existed in a top layer, in which case, it's a modification
			for _, layer := range layers {
				stat, err := os.Stat(filepath.Join(layer, path))
				if err != nil && !os.IsNotExist(err) {
					return err
				}
				if err == nil {
					// The file existed in the top layer, so that's a modification

					// However, if it's a directory, maybe it wasn't actually modified.
					// If you modify /foo/bar/baz, then /foo will be part of the changed files only because it's the parent of bar
					if stat.IsDir() && f.IsDir() {
						if f.Size() == stat.Size() && f.Mode() == stat.Mode() && f.ModTime() == stat.ModTime() {
							// Both directories are the same, don't record the change
							return nil
						}
					}
					change.Kind = ChangeModify
					break
				}
			}
		}

		// Record change
		changes = append(changes, change)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return changes, nil
}

func ChangesDirs(newDir, oldDir string) ([]Change, error) {
	var changes []Change
	err := filepath.Walk(newDir, func(newPath string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		var newStat syscall.Stat_t
		err = syscall.Lstat(newPath, &newStat)
		if err != nil {
			return err
		}

		// Rebase path
		relPath, err := filepath.Rel(newDir, newPath)
		if err != nil {
			return err
		}
		relPath = filepath.Join("/", relPath)

		// Skip root
		if relPath == "/" || relPath == "/.docker-id" {
			return nil
		}

		change := Change{
			Path: relPath,
		}

		oldPath := filepath.Join(oldDir, relPath)

		var oldStat = &syscall.Stat_t{}
		err = syscall.Lstat(oldPath, oldStat)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			oldStat = nil
		}

		if oldStat == nil {
			change.Kind = ChangeAdd
			changes = append(changes, change)
		} else {
			if oldStat.Ino != newStat.Ino ||
				oldStat.Mode != newStat.Mode ||
				oldStat.Uid != newStat.Uid ||
				oldStat.Gid != newStat.Gid ||
				oldStat.Rdev != newStat.Rdev ||
				oldStat.Size != newStat.Size ||
				oldStat.Blocks != newStat.Blocks ||
				oldStat.Mtim != newStat.Mtim ||
				oldStat.Ctim != newStat.Ctim {
				change.Kind = ChangeModify
				changes = append(changes, change)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	err = filepath.Walk(oldDir, func(oldPath string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Rebase path
		relPath, err := filepath.Rel(oldDir, oldPath)
		if err != nil {
			return err
		}
		relPath = filepath.Join("/", relPath)

		// Skip root
		if relPath == "/" {
			return nil
		}

		change := Change{
			Path: relPath,
		}

		newPath := filepath.Join(newDir, relPath)

		var newStat = &syscall.Stat_t{}
		err = syscall.Lstat(newPath, newStat)
		if err != nil && os.IsNotExist(err) {
			change.Kind = ChangeDelete
			changes = append(changes, change)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return changes, nil
}


func ExportChanges(root, rw string) (Archive, error) {
        changes, err := ChangesDirs(root, rw)
        if err != nil {
                return nil, err
        }
        files := make([]string, 0)
        deletions := make([]string, 0)
        for _, change := range changes {
                if change.Kind == ChangeModify || change.Kind == ChangeAdd {
                        files = append(files, change.Path)
                }
                if change.Kind == ChangeDelete {
                        base := filepath.Base(change.Path)
                        dir := filepath.Dir(change.Path)
                        deletions = append(deletions, filepath.Join(dir, ".wh."+base))
                }
        }
        return TarFilter(root, Uncompressed, files, false, deletions)
}

