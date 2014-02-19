package namespaces

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
)

const (
	SIGCHLD       = 0x14
	CLONE_VFORK   = 0x00004000
	CLONE_NEWNS   = 0x00020000
	CLONE_NEWUTS  = 0x04000000
	CLONE_NEWIPC  = 0x08000000
	CLONE_NEWUSER = 0x10000000
	CLONE_NEWPID  = 0x20000000
	CLONE_NEWNET  = 0x40000000
)

var namespaceMap = map[libcontainer.Namespace]int{
	"": 0,
	libcontainer.CLONE_NEWNS:   CLONE_NEWNS,
	libcontainer.CLONE_NEWUTS:  CLONE_NEWUTS,
	libcontainer.CLONE_NEWIPC:  CLONE_NEWIPC,
	libcontainer.CLONE_NEWUSER: CLONE_NEWUSER,
	libcontainer.CLONE_NEWPID:  CLONE_NEWPID,
	libcontainer.CLONE_NEWNET:  CLONE_NEWNET,
}

var namespaceFileMap = map[libcontainer.Namespace]string{
	libcontainer.CLONE_NEWNS:   "mnt",
	libcontainer.CLONE_NEWUTS:  "uts",
	libcontainer.CLONE_NEWIPC:  "ipc",
	libcontainer.CLONE_NEWUSER: "user",
	libcontainer.CLONE_NEWPID:  "pid",
	libcontainer.CLONE_NEWNET:  "net",
}
