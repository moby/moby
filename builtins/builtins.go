package builtins

import (
	"github.com/dotcloud/docker/engine"

	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/networkdriver/lxc"
)

func Register(eng *engine.Engine) {
	daemon(eng)
	remote(eng)
}

// remote: a RESTful api for cross-docker communication
func remote(eng *engine.Engine) {
	eng.Register("serveapi", api.ServeApi)
}

// daemon: a default execution and storage backend for Docker on Linux,
// with the following underlying components:
//
// * Pluggable storage drivers including aufs, vfs, lvm and btrfs.
// * Pluggable execution drivers including lxc and chroot.
//
// In practice `daemon` still includes most core Docker components, including:
//
// * The reference registry client implementation
// * Image management
// * The build facility
// * Logging
//
// These components should be broken off into plugins of their own.
//
func daemon(eng *engine.Engine) {
	eng.Register("initserver", docker.InitServer)
	eng.Register("init_networkdriver", lxc.InitDriver)
}
