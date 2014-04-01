package libcontainer

import (
	"syscall"
)

func init() {
	namespaceList = Namespaces{
		{Key: "NEWNS", Value: syscall.CLONE_NEWNS, File: "mnt", Enabled: true},
		{Key: "NEWUTS", Value: syscall.CLONE_NEWUTS, File: "uts", Enabled: true},
		{Key: "NEWIPC", Value: syscall.CLONE_NEWIPC, File: "ipc", Enabled: true},
		{Key: "NEWUSER", Value: syscall.CLONE_NEWUSER, File: "user", Enabled: true},
		{Key: "NEWPID", Value: syscall.CLONE_NEWPID, File: "pid", Enabled: true},
		{Key: "NEWNET", Value: syscall.CLONE_NEWNET, File: "net", Enabled: true},
	}
}
