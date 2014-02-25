package nsinit

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
)

// getNamespaceFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare, and setns
func GetNamespaceFlags(namespaces libcontainer.Namespaces) (flag int) {
	for _, ns := range namespaces {
		flag |= ns.Value
	}
	return flag
}
