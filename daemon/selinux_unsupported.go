// +build !linux

package daemon // import "github.com/moby/moby/daemon"

func selinuxSetDisabled() {
}

func selinuxFreeLxcContexts(label string) {
}

func selinuxEnabled() bool {
	return false
}
