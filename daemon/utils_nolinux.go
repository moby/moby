// +build !linux

package daemon

func selinuxSetDisabled() {
}

func selinuxFreeLxcContexts(label string) {
}
