// +build linux,cgo

package copy // import "github.com/docker/docker/daemon/graphdriver/copy"

/*
#include <linux/fs.h>

#ifndef FICLONE
#define FICLONE		_IOW(0x94, 9, int)
#endif
*/
import "C"
import (
	"os"

	"golang.org/x/sys/unix"
)

func fiClone(srcFile, dstFile *os.File) error {
	_, _, err := unix.Syscall(unix.SYS_IOCTL, dstFile.Fd(), C.FICLONE, srcFile.Fd())
	return err
}
