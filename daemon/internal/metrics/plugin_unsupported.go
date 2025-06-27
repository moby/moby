//go:build windows

package metrics

import (
	"github.com/docker/docker/daemon/pkg/plugin"
	"github.com/docker/docker/pkg/plugingetter"
)

func RegisterPlugin(*plugin.Store, string) error { return nil }
func CleanupPlugin(plugingetter.PluginGetter)    {}
