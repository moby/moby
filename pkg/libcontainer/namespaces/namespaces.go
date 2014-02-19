/*
   TODO
   pivot root
   cgroups
   more mount stuff that I probably am forgetting
   apparmor
*/

package namespaces

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
)

// JoinExistingNamespace uses the fd of an existing linux namespace and
// has the current process join that namespace or the spacespace specified by ns
func JoinExistingNamespace(fd uintptr, ns libcontainer.Namespace) error {
	flag := namespaceMap[ns]
	if err := Setns(fd, uintptr(flag)); err != nil {
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
