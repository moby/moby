package main

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
	"syscall"
)

var namespaceMap = map[libcontainer.Namespace]int{
	libcontainer.CLONE_NEWNS:   syscall.CLONE_NEWNS,
	libcontainer.CLONE_NEWUTS:  syscall.CLONE_NEWUTS,
	libcontainer.CLONE_NEWIPC:  syscall.CLONE_NEWIPC,
	libcontainer.CLONE_NEWUSER: syscall.CLONE_NEWUSER,
	libcontainer.CLONE_NEWPID:  syscall.CLONE_NEWPID,
	libcontainer.CLONE_NEWNET:  syscall.CLONE_NEWNET,
}

// getNamespaceFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare, and setns
func getNamespaceFlags(namespaces libcontainer.Namespaces) (flag int) {
	for _, ns := range namespaces {
		flag |= namespaceMap[ns]
	}
	return flag
}
