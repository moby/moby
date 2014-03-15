package builtins

import (
	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/runtime/networkdriver/lxc"
	"github.com/dotcloud/docker/server"
	"github.com/dotcloud/docker/sysinit"
)

func Register(eng *engine.Engine) {
	daemon(eng)
	remote(eng)
	runtime(eng)
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
	eng.Register("initserver", server.InitServer)
	eng.Register("init_networkdriver", lxc.InitDriver)
}

// runtime: the container's execution runtime
func runtime(eng *engine.Engine) {
	eng.Register("sysinit", sysinit.SysInit)
	// HACK: this is for the lxc issue
	eng.Register("/.dockerinit", sysinit.SysInit)
}
