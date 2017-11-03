// +build windows

package daemon

import "github.com/moby/moby/pkg/plugingetter"

func registerMetricsPluginCallback(getter plugingetter.PluginGetter, sockPath string) {
}

func (daemon *Daemon) listenMetricsSock() (string, error) {
	return "", nil
}
