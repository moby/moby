// +build daemon,linux

package main

import (
	apiserver "github.com/docker/docker/api/server"
)

func setPlatformServerConfig(serverConfig *apiserver.ServerConfig, daemonCfg *daemon.Config) *apiserver.ServerConfig {
	serverConfig.SocketGroup = daemonCfg.SocketGroup
	return serverConfig
}
