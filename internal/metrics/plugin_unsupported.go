//go:build windows

package metrics

import (
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/plugin"
)

func RegisterPlugin(*plugin.Store, string) error { return nil }
func CleanupPlugin(plugingetter.PluginGetter)    {}
