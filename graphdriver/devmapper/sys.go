package devmapper

import (
	"os"
	"os/exec"
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

	osOpenFile = func(name string, flag int, perm os.FileMode) (*osFile, error) {
		f, err := os.OpenFile(name, flag, perm)
		return &osFile{File: f}, err
	}
	osOpen       = func(name string) (*osFile, error) { f, err := os.Open(name); return &osFile{File: f}, err }
	osNewFile    = os.NewFile
	osCreate     = os.Create
	osStat       = os.Stat
	osIsNotExist = os.IsNotExist
	osIsExist    = os.IsExist
	osMkdirAll   = os.MkdirAll
	osRemoveAll  = os.RemoveAll
	osRename     = os.Rename
	osReadlink   = os.Readlink

	execRun = func(name string, args ...string) error { return exec.Command(name, args...).Run() }
)

const (
	sysMsMgcVal = syscall.MS_MGC_VAL
	sysMsRdOnly = syscall.MS_RDONLY
	sysEInval   = syscall.EINVAL
	sysSysIoctl = syscall.SYS_IOCTL
	sysEBusy    = syscall.EBUSY

	osORdOnly    = os.O_RDONLY
	osORdWr      = os.O_RDWR
	osOCreate    = os.O_CREATE
	osModeDevice = os.ModeDevice
)

func toSysStatT(i interface{}) *sysStatT {
	return (*sysStatT)(i.(*syscall.Stat_t))
}
