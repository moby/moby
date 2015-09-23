// +build !daemon

package main

import "github.com/docker/docker/cli"

const daemonUsage = ""

var daemonCli cli.Handler

// TODO: remove once `-d` is retired
func handleGlobalDaemonFlag() {}

// notifySystem sends a message to the host when the server is ready to be used
func notifySystem() {
}
