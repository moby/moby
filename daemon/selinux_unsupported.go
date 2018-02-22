// +build !linux

package daemon // import "github.com/docker/docker/daemon"

func selinuxSetDisabled() {
}

func selinuxFreeLxcContexts(label string) {
}

func selinuxEnabled() bool {
	return false
}
