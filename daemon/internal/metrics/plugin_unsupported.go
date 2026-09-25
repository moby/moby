//go:build windows

package metrics

import (
	"github.com/moby/moby/v2/daemon/pkg/plugin"
	"github.com/moby/moby/v2/pkg/plugingetter"
)

func RegisterPlugin(*plugin.Store, string) error { return nil }
func CleanupPlugin(plugingetter.PluginGetter)    {}
