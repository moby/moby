package devmapper

import (
	"os"
	"syscall"
)

type (
	sysStatT syscall.Stat_t
	sysErrno syscall.Errno
)

var (
	// functions
	sysMount       = syscall.Mount
	sysUnmount     = syscall.Unmount
	sysCloseOnExec = syscall.CloseOnExec
	sysSyscall     = syscall.Syscall
	osOpenFile     = os.OpenFile
)

const (
	sysMsMgcVal = syscall.MS_MGC_VAL
	sysMsRdOnly = syscall.MS_RDONLY
	sysEInval   = syscall.EINVAL
	sysSysIoctl = syscall.SYS_IOCTL
)

func toSysStatT(i interface{}) *sysStatT {
	return (*sysStatT)(i.(*syscall.Stat_t))
}
