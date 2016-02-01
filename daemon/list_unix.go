// +build linux freebsd

package daemon

import "github.com/docker/engine-api/types/container"

// excludeByIsolation is a platform specific helper function to support PS
// filtering by Isolation. This is a Windows-only concept, so is a no-op on Unix.
func excludeByIsolation(isolation container.IsolationLevel, ctx *listContext) bool {
	return false
}
