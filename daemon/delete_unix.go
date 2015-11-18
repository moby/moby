// +build linux freebsd

package daemon

// platformSpecificRm is a platform-specific helper function for rm/delete
func platformSpecificRm(container *Container) {
	selinuxFreeLxcContexts(container.ProcessLabel)
}
