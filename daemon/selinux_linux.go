// +build linux

package daemon

import selinux "github.com/opencontainers/selinux/go-selinux"

func selinuxSetDisabled() {
	selinux.SetDisabled()
}

func selinuxReleaseLabel(label string) {
	selinux.ReleaseLabel(label)
}

func selinuxEnabled() bool {
	return selinux.GetEnabled()
}
