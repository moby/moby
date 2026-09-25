package daemon

import (
	"github.com/containerd/containerd/v2/pkg/apparmor"
	"github.com/moby/moby/v2/daemon/internal/rootless"
)

// appArmorSupported returns true if AppArmor is supported and accessible on the host.
func appArmorSupported() bool {
	if detachedNetNS, _ := rootless.DetachedNetNS(); detachedNetNS != "" {
		// AppArmor is inaccessible with detached-netns because sysfs is netns-scoped.
		// https://github.com/moby/moby/issues/52626
		return false
	}
	return apparmor.HostSupports()
}
