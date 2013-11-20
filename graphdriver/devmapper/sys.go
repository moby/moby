package devmapper


import (
	"syscall"
)


var (
	SyscallMount		= syscall.Mount
	SyscallUnmount		= syscall.Unmount
	SyscallCloseOnExec	= syscall.CloseOnExec
	SyscallSyscall		= syscall.Syscall
	OSOpenFile		= os.OpenFile
)
