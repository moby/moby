package dockerversion

// FIXME: this should be embedded in the docker/docker.go,
// but we can't because distro policy requires us to
// package a separate dockerinit binary, and that binary needs
// to know its version too.

var (
	GITCOMMIT string
	VERSION   string
)
