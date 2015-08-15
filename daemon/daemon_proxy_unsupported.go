// +build !linux

package daemon

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/mflag"
)

type ProxyConfig struct {
	dummy bool
}

func (config *ProxyConfig) InstallProxyFlags(cmd *mflag.FlagSet, usageFn func(string) string) {
	cmd.BoolVar(&config.dummy, []string{"dummy"}, true, usageFn("dummy"))
}

func StartProxyDaemon(config *ProxyConfig, cli *cli.Cli) {
	logrus.Fatal("Unsupported platform")
}
