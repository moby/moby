package daemon

import (
	"strings"

	"github.com/docker/engine-api/types/container"
)

// excludeByIsolation is a platform specific helper function to support PS
// filtering by Isolation. This is a Windows-only concept, so is a no-op on Unix.
func excludeByIsolation(isolation container.IsolationLevel, ctx *listContext) bool {
	i := strings.ToLower(string(isolation))
	if i == "" {
		i = "default"
	}
	return !ctx.filters.Match("isolation", i)
}
