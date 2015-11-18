// +build !linux

package daemon

func selinuxSetDisabled() {
}

func selinuxEnabled() bool {
	return false
}
