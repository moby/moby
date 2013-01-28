package docker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type Filesystem struct {
	RootFS string
	RWPath string
	Layers []string
}

func (fs *Filesystem) createMountPoints() error {
	if err := os.Mkdir(fs.RootFS, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	if err := os.Mkdir(fs.RWPath, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func (fs *Filesystem) Mount() error {
	if fs.IsMounted() {
		return errors.New("Mount: Filesystem already mounted")
	}
	if err := fs.createMountPoints(); err != nil {
		return err
	}
	rwBranch := fmt.Sprintf("%v=rw", fs.RWPath)
	roBranches := ""
	for _, layer := range fs.Layers {
		roBranches += fmt.Sprintf("%v=ro:", layer)
	}
	branches := fmt.Sprintf("br:%v:%v", rwBranch, roBranches)
	if err := syscall.Mount("none", fs.RootFS, "aufs", 0, branches); err != nil {
		return err
	}
	if !fs.IsMounted() {
		return errors.New("Mount failed")
	}
	return nil
}

func (fs *Filesystem) Umount() error {
	if !fs.IsMounted() {
		return errors.New("Umount: Filesystem not mounted")
	}
	if err := syscall.Unmount(fs.RootFS, 0); err != nil {
		return err
	}
	if fs.IsMounted() {
		return fmt.Errorf("Umount: Filesystem still mounted after calling umount(%v)", fs.RootFS)
	}
	// Even though we just unmounted the filesystem, AUFS will prevent deleting the mntpoint
	// for some time. We'll just keep retrying until it succeeds.
	for retries := 0; retries < 1000; retries++ {
		err := os.Remove(fs.RootFS)
		if err == nil {
			// rm mntpoint succeeded
			return nil
		}
		if os.IsNotExist(err) {
			// mntpoint doesn't exist anymore. Success.
			return nil
		}
		// fmt.Printf("(%v) Remove %v returned: %v\n", retries, fs.RootFS, err)
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("Umount: Failed to umount %v", fs.RootFS)
}

func (fs *Filesystem) IsMounted() bool {
	mntpoint, err := os.Stat(fs.RootFS)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		panic(err)
	}
	parent, err := os.Stat(filepath.Join(fs.RootFS, ".."))
	if err != nil {
		panic(err)
	}

	mntpointSt := mntpoint.Sys().(*syscall.Stat_t)
	parentSt := parent.Sys().(*syscall.Stat_t)
	return mntpointSt.Dev != parentSt.Dev
}

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

func (fs *Filesystem) Changes() ([]Change, error) {
	var changes []Change
	err := filepath.Walk(fs.RWPath, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Rebase path
		path, err = filepath.Rel(fs.RWPath, path)
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
			originalFile := strings.TrimLeft(file, ".wh.")
			change.Path = filepath.Join(filepath.Dir(path), originalFile)
			change.Kind = ChangeDelete
		} else {
			// Otherwise, the file was added
			change.Kind = ChangeAdd

			// ...Unless it already existed in a top layer, in which case, it's a modification
			for _, layer := range fs.Layers {
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
	if err != nil {
		return nil, err
	}
	return changes, nil
}

func newFilesystem(rootfs string, rwpath string, layers []string) *Filesystem {
	return &Filesystem{
		RootFS: rootfs,
		RWPath: rwpath,
		Layers: layers,
	}
}
