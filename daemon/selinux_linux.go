package daemon // import "github.com/docker/docker/daemon"

import selinux "github.com/opencontainers/selinux/go-selinux"

func selinuxSetDisabled() {
	selinux.SetDisabled()
}

func selinuxFreeLxcContexts(label string) {
	selinux.ReleaseLabel(label)
}

func selinuxEnabled() bool {
	return selinux.GetEnabled()
}
