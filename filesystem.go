package docker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"io"
	"io/ioutil"
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
	return syscall.Mount("none", fs.RootFS, "aufs", 0, branches)
}

func (fs *Filesystem) Umount() error {
	if !fs.IsMounted() {
		return errors.New("Umount: Filesystem not mounted")
	}
	return syscall.Unmount(fs.RootFS, 0)
}

func (fs *Filesystem) IsMounted() bool {
	f, err := os.Open(fs.RootFS)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		panic(err)
	}
	list, err := f.Readdirnames(1)
	f.Close()
	if err != nil {
		return false
	}
	if len(list) > 0 {
		return true
	}
	return false
}

// Tar returns the contents of the filesystem as an uncompressed tar stream
func (fs *Filesystem) Tar() (io.Reader, error) {
	if err := fs.EnsureMounted(); err != nil {
		return nil, err
	}
	return Tar(fs.RootFS)
}


func (fs *Filesystem) EnsureMounted() error {
	if !fs.IsMounted() {
		if err := fs.Mount(); err != nil {
			return err
		}
	}
	return nil
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

func (change *Change) String() string {
	var kind string
	switch change.Kind {
		case ChangeModify:	kind = "C"
		case ChangeAdd:		kind = "A"
		case ChangeDelete:	kind = "D"
	}
	return fmt.Sprintf("%s %s", kind, change.Path)
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

// Reset removes all changes to the filesystem, reverting it to its initial state.
func (fs *Filesystem) Reset() error {
	if err := os.RemoveAll(fs.RWPath); err != nil {
		return err
	}
	// We removed the RW directory itself along with its content: let's re-create an empty one.
	if err := fs.createMountPoints(); err != nil {
		return err
	}
	return nil
}

// Open opens the named file for reading.
func (fs *Filesystem) OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	if err := fs.EnsureMounted(); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(fs.RootFS, path), flag, perm)
}

// ReadDir reads the directory named by dirname, relative to the Filesystem's root,
// and returns a list of sorted directory entries
func (fs *Filesystem) ReadDir(dirname string) ([]os.FileInfo, error) {
	if err := fs.EnsureMounted(); err != nil {
		return nil, err
	}
	return ioutil.ReadDir(filepath.Join(fs.RootFS, dirname))
}

func newFilesystem(rootfs string, rwpath string, layers []string) *Filesystem {
	return &Filesystem{
		RootFS: rootfs,
		RWPath: rwpath,
		Layers: layers,
	}
}
