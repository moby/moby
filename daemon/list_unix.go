// +build linux freebsd

package daemon

// excludeByIsolation is a platform specific helper function to support PS
// filtering by Isolation. This is a Windows-only concept, so is a no-op on Unix.
func excludeByIsolation(container *Container, ctx *listContext) iterationAction {
	return includeContainer
}
