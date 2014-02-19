/*
   TODO
   pivot root
   cgroups
   more mount stuff that I probably am forgetting
   apparmor
*/

package namespaces

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/utils"
	"os"
	"path/filepath"
	"syscall"
)

// CreateNewNamespace creates a new namespace and binds it's fd to the specified path
func CreateNewNamespace(namespace libcontainer.Namespace, bindTo string) error {
	var (
		flag   = namespaceMap[namespace]
		name   = namespaceFileMap[namespace]
		nspath = filepath.Join("/proc/self/ns", name)
	)
	// TODO: perform validation on name and flag

	pid, err := fork()
	if err != nil {
		return err
	}

	if pid == 0 {
		if err := unshare(flag); err != nil {
			writeError("unshare %s", err)
		}
		if err := mount(nspath, bindTo, "none", syscall.MS_BIND, ""); err != nil {
			writeError("bind mount %s", err)
		}
		os.Exit(0)
	}
	exit, err := utils.WaitOnPid(pid)
	if err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("exit status %d", exit)
	}
	return err
}

// JoinExistingNamespace uses the fd of an existing linux namespace and
// has the current process join that namespace or the spacespace specified by ns
func JoinExistingNamespace(fd uintptr, ns libcontainer.Namespace) error {
	flag := namespaceMap[ns]
	if err := setns(fd, uintptr(flag)); err != nil {
		return err
	}
	return nil
}

// getNamespaceFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare, and setns
func getNamespaceFlags(namespaces libcontainer.Namespaces) (flag int) {
	for _, ns := range namespaces {
		flag |= namespaceMap[ns]
	}
	return
}
