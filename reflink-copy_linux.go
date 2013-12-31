package docker

// FIXME: This could be easily rewritten in pure Go

/*
#include <sys/ioctl.h>
#include <linux/fs.h>
#include <errno.h>

// See linux.git/fs/btrfs/ioctl.h
#define BTRFS_IOCTL_MAGIC 0x94
#define BTRFS_IOC_CLONE _IOW(BTRFS_IOCTL_MAGIC, 9, int)

int
btrfs_reflink(int fd_out, int fd_in)
{
  int res;
  res = ioctl(fd_out, BTRFS_IOC_CLONE, fd_in);
  if (res < 0)
    return errno;
  return 0;
}

*/
import "C"

import (
	"os"
	"io"
	"syscall"
)

// FIXME: Move this to btrfs package?

func BtrfsReflink(fd_out, fd_in uintptr) error {
	res := C.btrfs_reflink(C.int(fd_out), C.int(fd_in))
	if res != 0 {
		return syscall.Errno(res)
	}
	return nil
}

func CopyFile(dstFile, srcFile *os.File) error {
	err := BtrfsReflink(dstFile.Fd(), srcFile.Fd())
	if err == nil {
		return nil
	}

	// Fall back to normal copy
	// FIXME: Check the return of Copy and compare with dstFile.Stat().Size
	_, err = io.Copy(dstFile, srcFile)
	return err
}
