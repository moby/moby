//go:build windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/plugingetter"
)

func registerMetricsPluginCallback(getter plugingetter.PluginGetter, sockPath string) {
}

func (daemon *Daemon) listenMetricsSock(*config.Config) (string, error) {
	return "", nil
}
