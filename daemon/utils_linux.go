// +build linux

package daemon

import "github.com/docker/libcontainer/selinux"

func selinuxSetDisabled() {
	selinux.SetDisabled()
}

func selinuxFreeLxcContexts(label string) {
	selinux.FreeLxcContexts(label)
}

func selinuxEnabled() bool {
	return selinux.SelinuxEnabled()
}
