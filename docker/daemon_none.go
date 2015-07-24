// +build !daemon

package main

import "github.com/docker/docker/cli"

const daemonUsage = ""

var daemonCli cli.Handler

// TODO: remove once `-d` is retired
func handleGlobalDaemonFlag() {}
