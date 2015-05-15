// +build daemon,!windows

package main

import (
	"os"

	apiserver "github.com/docker/docker/api/server"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/system"

	_ "github.com/docker/docker/daemon/execdriver/lxc"
	_ "github.com/docker/docker/daemon/execdriver/native"
)

func setPlatformServerConfig(serverConfig *apiserver.ServerConfig, daemonCfg *daemon.Config) *apiserver.ServerConfig {
	serverConfig.SocketGroup = daemonCfg.SocketGroup
	return serverConfig
}

// currentUserIsOwner checks whether the current user is the owner of the given
// file.
func currentUserIsOwner(f string) bool {
	if fileInfo, err := system.Stat(f); err == nil && fileInfo != nil {
		if int(fileInfo.Uid()) == os.Getuid() {
			return true
		}
	}
	return false
}
