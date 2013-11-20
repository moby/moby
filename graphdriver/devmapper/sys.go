package devmapper

import (
	"os"
	"syscall"
)

type (
	sysStatT syscall.Stat_t
	sysErrno syscall.Errno

	osFile struct{ *os.File }
)

var (
	sysMount       = syscall.Mount
	sysUnmount     = syscall.Unmount
	sysCloseOnExec = syscall.CloseOnExec
	sysSyscall     = syscall.Syscall

	osOpenFile   = os.OpenFile
	osNewFile    = os.NewFile
	osCreate     = os.Create
	osStat       = os.Stat
	osIsNotExist = os.IsNotExist
	osIsExist    = os.IsExist
	osMkdirAll   = os.MkdirAll
	osRemoveAll  = os.RemoveAll
	osRename     = os.Rename
	osReadlink   = os.Readlink
)

const (
	sysMsMgcVal = syscall.MS_MGC_VAL
	sysMsRdOnly = syscall.MS_RDONLY
	sysEInval   = syscall.EINVAL
	sysSysIoctl = syscall.SYS_IOCTL

	osORdWr   = os.O_RDWR
	osOCreate = os.O_CREATE
)

func toSysStatT(i interface{}) *sysStatT {
	return (*sysStatT)(i.(*syscall.Stat_t))
}
