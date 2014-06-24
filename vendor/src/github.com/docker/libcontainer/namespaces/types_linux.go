package namespaces

import (
	"syscall"
)

func init() {
	namespaceList = Namespaces{
		{Key: "NEWNS", Value: syscall.CLONE_NEWNS, File: "mnt"},
		{Key: "NEWUTS", Value: syscall.CLONE_NEWUTS, File: "uts"},
		{Key: "NEWIPC", Value: syscall.CLONE_NEWIPC, File: "ipc"},
		{Key: "NEWUSER", Value: syscall.CLONE_NEWUSER, File: "user"},
		{Key: "NEWPID", Value: syscall.CLONE_NEWPID, File: "pid"},
		{Key: "NEWNET", Value: syscall.CLONE_NEWNET, File: "net"},
	}
}
