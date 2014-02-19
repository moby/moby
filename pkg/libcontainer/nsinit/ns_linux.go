package main

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

// getNamespaceFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare, and setns
func getNamespaceFlags(namespaces libcontainer.Namespaces) (flag int) {
	for _, ns := range namespaces {
		flag |= namespaceMap[ns]
	}
	return flag
}
