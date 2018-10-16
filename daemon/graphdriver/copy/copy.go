// +build linux

package copy // import "github.com/docker/docker/daemon/graphdriver/copy"

/*
#include <linux/fs.h>

#ifndef FICLONE
#define FICLONE		_IOW(0x94, 9, int)
#endif
*/
import "C"
import (
	"container/list"
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/pools"
	"github.com/docker/docker/pkg/system"
	rsystem "github.com/opencontainers/runc/libcontainer/system"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

const (
	// Docker sets this earlier on in initialization to 0022
	// Unfortunately, sometimes the special bits are not respected
	// and we have to force-set them.
	desiredUmask = 7022
	// This is based on some empirical testing on EXT4 filesystems
	workerCount              = 8
	fileCopyInfoChannelDepth = 100
)

// Mode indicates whether to use hardlink or copy content
type Mode int

const (
	// Content creates a new file, and copies the content of the file
	Content Mode = iota
	// Hardlink creates a new hardlink to the existing file
	Hardlink
)

func copyRegular(srcPath, dstPath string, fileinfo os.FileInfo, copyWithFileRange, copyWithFileClone *bool) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// If the destination file already exists, we shouldn't blow it away
	dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileinfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if *copyWithFileClone {
		_, _, err = unix.Syscall(unix.SYS_IOCTL, dstFile.Fd(), C.FICLONE, srcFile.Fd())
		if err == nil {
			return nil
		}

		*copyWithFileClone = false
		if err == unix.EXDEV {
			*copyWithFileRange = false
		}
	}
	if *copyWithFileRange {
		err = doCopyWithFileRange(srcFile, dstFile, fileinfo)
		// Trying the file_clone may not have caught the exdev case
		// as the ioctl may not have been available (therefore EINVAL)
		if err == unix.EXDEV || err == unix.ENOSYS {
			*copyWithFileRange = false
		} else {
			return err
		}
	}
	return legacyCopy(srcFile, dstFile)
}

func doCopyWithFileRange(srcFile, dstFile *os.File, fileinfo os.FileInfo) error {
	amountLeftToCopy := fileinfo.Size()

	for amountLeftToCopy > 0 {
		n, err := unix.CopyFileRange(int(srcFile.Fd()), nil, int(dstFile.Fd()), nil, int(amountLeftToCopy), 0)
		if err != nil {
			return err
		}

		amountLeftToCopy = amountLeftToCopy - int64(n)
	}

	return nil
}

func legacyCopy(srcFile io.Reader, dstFile io.Writer) error {
	_, err := pools.Copy(dstFile, srcFile)

	return err
}

func copyXattr(srcPath, dstPath, attr string) error {
	data, err := system.Lgetxattr(srcPath, attr)
	if err != nil {
		return err
	}
	if data != nil {
		if err := system.Lsetxattr(dstPath, attr, data, 0); err != nil {
			return err
		}
	}
	return nil
}

type dirMtimeInfo struct {
	dstPath *string
	stat    *syscall.Stat_t
}

type fileCopyInfo struct {
	dir      bool
	dstPath  string
	srcPath  string
	stat     *syscall.Stat_t
	fileInfo os.FileInfo
}

type fileID struct {
	dev uint64
	ino uint64
}

// This represents an inode on the destination side with a unique identity
type sharedInode struct {
	wg                        *sync.WaitGroup
	successfullyCopiedDstPath string
	err                       error
}

// This tells us about the shared dstPath
func newSharedInode(dstPath string) *sharedInode {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return &sharedInode{wg: wg, successfullyCopiedDstPath: dstPath}
}

func doFileCopy(srcPath, dstPath string, f os.FileInfo, stat *syscall.Stat_t, duplicatedInodeMap *sync.Map, copyWithFileRange, copyWithFileClone *bool, copyXattrs bool, copyMode Mode) error {
	switch f.Mode() & os.ModeType {
	case 0: // Regular file
		if copyMode == Hardlink {
			if err := os.Link(srcPath, dstPath); err != nil {
				return err
			}
			// Don't copy metadata for hard links
			return nil
		}
		id := fileID{dev: stat.Dev, ino: stat.Ino}
		sharedInodeInterface, loaded := duplicatedInodeMap.LoadOrStore(id, newSharedInode(dstPath))
		inode := sharedInodeInterface.(*sharedInode)
		if loaded {
			inode.wg.Wait()
			if inode.err != nil {
				return inode.err
			}
			if err := os.Link(inode.successfullyCopiedDstPath, dstPath); err != nil {
				return err
			}
		} else {
			inode.err = copyRegular(srcPath, dstPath, f, copyWithFileRange, copyWithFileClone)
			inode.wg.Done()
			if inode.err != nil {
				return inode.err
			}
		}

	case os.ModeSymlink:
		link, err := os.Readlink(srcPath)
		if err != nil {
			return err
		}

		if err := os.Symlink(link, dstPath); err != nil {
			return err
		}

	case os.ModeNamedPipe:
		fallthrough
	case os.ModeSocket:
		if err := unix.Mkfifo(dstPath, stat.Mode); err != nil {
			return err
		}

	case os.ModeDevice:
		if rsystem.RunningInUserNS() {
			// cannot create a device if running in user namespace
			return nil
		}
		if err := unix.Mknod(dstPath, stat.Mode, int(stat.Rdev)); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unknown file type for %s", srcPath)
	}

	return copyMetadata(dstPath, srcPath, f, stat, copyXattrs)
}

func fileCopyWorker(ctx context.Context, fileCopyInfoChan chan *fileCopyInfo, duplicatedInodeMap *sync.Map, copyMode Mode, copyXattrs bool) error {
	copyWithFileRange := true
	copyWithFileClone := true

	// Just a dumb wrapper around the above function which does the heavy lifting
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case fci, ok := <-fileCopyInfoChan:
			if !ok {
				return nil
			}
			if fci.dir {
				if err := copyMetadataShared(fci.dstPath, fci.srcPath, fci.fileInfo, fci.stat, copyXattrs); err != nil {
					return err
				}

			} else {
				if err := doFileCopy(fci.srcPath, fci.dstPath, fci.fileInfo, fci.stat, duplicatedInodeMap, &copyWithFileRange, &copyWithFileClone, copyXattrs, copyMode); err != nil {
					return err
				}
			}
		}

	}
}

// DirCopy copies or hardlinks the contents of one directory to another,
// properly handling xattrs, and soft links
//
// Copying xattrs can be opted out of by passing false for copyXattrs.
func DirCopy(srcDir, dstDir string, copyMode Mode, copyXattrs bool) error {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	dirsToSetMtimes := list.New()
	errGroup, errGroupCtx := errgroup.WithContext(ctx)
	workers := make([]chan *fileCopyInfo, workerCount)
	duplicatedInodeMap := &sync.Map{}
	for i := 0; i < len(workers); i++ {
		c := make(chan *fileCopyInfo, fileCopyInfoChannelDepth)
		workers[i] = c
		errGroup.Go(func() error {
			return fileCopyWorker(errGroupCtx, c, duplicatedInodeMap, copyMode, copyXattrs)
		})
	}

	err := filepath.Walk(srcDir, func(srcPath string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if the errgroup has gotten canceled / shutdown
		if errGroupCtx.Err() != nil {
			return nil
		}

		// Rebase path
		relPath, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dstDir, relPath)
		if err != nil {
			return err
		}

		stat, ok := f.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("Unable to get raw syscall.Stat_t data for %s", srcPath)
		}
		if f.IsDir() {
			if err := os.Mkdir(dstPath, f.Mode()); err != nil && !os.IsExist(err) {
				return err
			}
			if f.Mode()&desiredUmask > 0 {
				if err := os.Chmod(dstPath, f.Mode()); err != nil {
					return err
				}
			}
			dirsToSetMtimes.PushFront(&dirMtimeInfo{dstPath: &dstPath, stat: stat})
			// Only copy a subset of the metadata, but don't update timestamps until the very
			// end (in reverse)
		}

		workers[rand.Intn(len(workers))] <- &fileCopyInfo{
			dir:      f.IsDir(),
			dstPath:  dstPath,
			srcPath:  srcPath,
			stat:     stat,
			fileInfo: f,
		}
		return nil
	})
	for i := 0; i < len(workers); i++ {
		close(workers[i])
	}
	if err != nil {
		return err
	}

	err = errGroup.Wait()
	if err != nil {
		return err
	}

	for e := dirsToSetMtimes.Front(); e != nil; e = e.Next() {
		mtimeInfo := e.Value.(*dirMtimeInfo)
		ts := []syscall.Timespec{mtimeInfo.stat.Atim, mtimeInfo.stat.Mtim}
		if err := system.LUtimesNano(*mtimeInfo.dstPath, ts); err != nil {
			return err
		}
	}
	return nil
}

// copyMetadataShared copies the metadata for directories and files, but doesn't
// set atimes / mtimes because those have to happen after all file modifications
// are complete
func copyMetadataShared(dstPath, srcPath string, f os.FileInfo, stat *syscall.Stat_t, copyXattrs bool) error {
	if err := os.Lchown(dstPath, int(stat.Uid), int(stat.Gid)); err != nil {
		return err
	}

	if copyXattrs {
		if err := doCopyXattrs(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

func copyMetadata(dstPath, srcPath string, f os.FileInfo, stat *syscall.Stat_t, copyXattrs bool) error {
	if err := copyMetadataShared(dstPath, srcPath, f, stat, copyXattrs); err != nil {
		return err
	}
	isSymlink := f.Mode()&os.ModeSymlink != 0
	if isSymlink {
		// There is no LChmod, so ignore mode for symlink. Also, this
		// must happen after chown, as that can modify the file mode
		ts := []syscall.Timespec{stat.Atim, stat.Mtim}
		if err := system.LUtimesNano(dstPath, ts); err != nil {
			return err
		}
	} else {
		if f.Mode()&desiredUmask > 0 {
			if err := os.Chmod(dstPath, f.Mode()); err != nil {
				return err
			}
		}
		aTime := time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
		mTime := time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec)
		if err := system.Chtimes(dstPath, aTime, mTime); err != nil {
			return err
		}
	}

	return nil
}

func doCopyXattrs(srcPath, dstPath string) error {
	if err := copyXattr(srcPath, dstPath, "security.capability"); err != nil {
		return err
	}

	// We need to copy this attribute if it appears in an overlay upper layer, as
	// this function is used to copy those. It is set by overlay if a directory
	// is removed and then re-created and should not inherit anything from the
	// same dir in the lower dir.
	return copyXattr(srcPath, dstPath, "trusted.overlay.opaque")
}
