package pathfs

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

type loopbackFileSystem struct {
	// TODO - this should need default fill in.
	FileSystem
	Root string
}

// A FUSE filesystem that shunts all request to an underlying file
// system.  Its main purpose is to provide test coverage without
// having to build a synthetic filesystem.
func NewLoopbackFileSystem(root string) FileSystem {
	return &loopbackFileSystem{
		FileSystem: NewDefaultFileSystem(),
		Root:       root,
	}
}

func (fs *loopbackFileSystem) OnMount(nodeFs *PathNodeFs) {
}

func (fs *loopbackFileSystem) OnUnmount() {}

func (fs *loopbackFileSystem) GetPath(relPath string) string {
	return filepath.Join(fs.Root, relPath)
}

func (fs *loopbackFileSystem) GetAttr(name string, context *fuse.Context) (a *fuse.Attr, code fuse.Status) {
	fullPath := fs.GetPath(name)
	var err error = nil
	st := syscall.Stat_t{}
	if name == "" {
		// When GetAttr is called for the toplevel directory, we always want
		// to look through symlinks.
		err = syscall.Stat(fullPath, &st)
	} else {
		err = syscall.Lstat(fullPath, &st)
	}

	if err != nil {
		return nil, fuse.ToStatus(err)
	}
	a = &fuse.Attr{}
	a.FromStat(&st)
	return a, fuse.OK
}

func (fs *loopbackFileSystem) OpenDir(name string, context *fuse.Context) (stream []fuse.DirEntry, status fuse.Status) {
	// What other ways beyond O_RDONLY are there to open
	// directories?
	f, err := os.Open(fs.GetPath(name))
	if err != nil {
		return nil, fuse.ToStatus(err)
	}
	want := 500
	output := make([]fuse.DirEntry, 0, want)
	for {
		infos, err := f.Readdir(want)
		for i := range infos {
			// workaround forhttps://code.google.com/p/go/issues/detail?id=5960
			if infos[i] == nil {
				continue
			}
			n := infos[i].Name()
			d := fuse.DirEntry{
				Name: n,
			}
			if s := fuse.ToStatT(infos[i]); s != nil {
				d.Mode = uint32(s.Mode)
			} else {
				log.Printf("ReadDir entry %q for %q has no stat info", n, name)
			}
			output = append(output, d)
		}
		if len(infos) < want || err == io.EOF {
			break
		}
		if err != nil {
			log.Println("Readdir() returned err:", err)
			break
		}
	}
	f.Close()

	return output, fuse.OK
}

func (fs *loopbackFileSystem) Open(name string, flags uint32, context *fuse.Context) (fuseFile nodefs.File, status fuse.Status) {
	f, err := os.OpenFile(fs.GetPath(name), int(flags), 0)
	if err != nil {
		return nil, fuse.ToStatus(err)
	}
	return nodefs.NewLoopbackFile(f), fuse.OK
}

func (fs *loopbackFileSystem) Chmod(path string, mode uint32, context *fuse.Context) (code fuse.Status) {
	err := os.Chmod(fs.GetPath(path), os.FileMode(mode))
	return fuse.ToStatus(err)
}

func (fs *loopbackFileSystem) Chown(path string, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ToStatus(os.Chown(fs.GetPath(path), int(uid), int(gid)))
}

func (fs *loopbackFileSystem) Truncate(path string, offset uint64, context *fuse.Context) (code fuse.Status) {
	return fuse.ToStatus(os.Truncate(fs.GetPath(path), int64(offset)))
}

func (fs *loopbackFileSystem) Readlink(name string, context *fuse.Context) (out string, code fuse.Status) {
	f, err := os.Readlink(fs.GetPath(name))
	return f, fuse.ToStatus(err)
}

func (fs *loopbackFileSystem) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ToStatus(syscall.Mknod(fs.GetPath(name), mode, int(dev)))
}

func (fs *loopbackFileSystem) Mkdir(path string, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ToStatus(os.Mkdir(fs.GetPath(path), os.FileMode(mode)))
}

// Don't use os.Remove, it removes twice (unlink followed by rmdir).
func (fs *loopbackFileSystem) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	return fuse.ToStatus(syscall.Unlink(fs.GetPath(name)))
}

func (fs *loopbackFileSystem) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	return fuse.ToStatus(syscall.Rmdir(fs.GetPath(name)))
}

func (fs *loopbackFileSystem) Symlink(pointedTo string, linkName string, context *fuse.Context) (code fuse.Status) {
	return fuse.ToStatus(os.Symlink(pointedTo, fs.GetPath(linkName)))
}

func (fs *loopbackFileSystem) Rename(oldPath string, newPath string, context *fuse.Context) (codee fuse.Status) {
	err := os.Rename(fs.GetPath(oldPath), fs.GetPath(newPath))
	return fuse.ToStatus(err)
}

func (fs *loopbackFileSystem) Link(orig string, newName string, context *fuse.Context) (code fuse.Status) {
	return fuse.ToStatus(os.Link(fs.GetPath(orig), fs.GetPath(newName)))
}

func (fs *loopbackFileSystem) Access(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ToStatus(syscall.Access(fs.GetPath(name), mode))
}

func (fs *loopbackFileSystem) Create(path string, flags uint32, mode uint32, context *fuse.Context) (fuseFile nodefs.File, code fuse.Status) {
	f, err := os.OpenFile(fs.GetPath(path), int(flags)|os.O_CREATE, os.FileMode(mode))
	return nodefs.NewLoopbackFile(f), fuse.ToStatus(err)
}
