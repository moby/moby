// +build !linux !amd64

package lxc

import "github.com/dotcloud/docker/daemon/execdriver"

func setHostname(hostname string) error {
	panic("Not supported on darwin")
}

func finalizeNamespace(args *execdriver.InitArgs) error {
	panic("Not supported on darwin")
}
