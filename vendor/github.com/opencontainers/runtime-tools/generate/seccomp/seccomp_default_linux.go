//go:build linux

package seccomp

import "golang.org/x/sys/unix"

// System values passed through on linux
const (
	CloneNewIPC    = unix.CLONE_NEWIPC
	CloneNewNet    = unix.CLONE_NEWNET
	CloneNewNS     = unix.CLONE_NEWNS
	CloneNewPID    = unix.CLONE_NEWPID
	CloneNewUser   = unix.CLONE_NEWUSER
	CloneNewUTS    = unix.CLONE_NEWUTS
	CloneNewCgroup = unix.CLONE_NEWCGROUP
)
