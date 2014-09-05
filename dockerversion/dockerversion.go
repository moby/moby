package dockerversion

// FIXME: this should be embedded in the docker/docker.go,
// but we can't because distro policy requires us to
// package a separate dockerinit binary, and that binary needs
// to know its version too.

var (
	GITCOMMIT string
	VERSION   string

	IAMSTATIC bool   // whether or not Docker itself was compiled statically via ./hack/make.sh binary
	INITSHA1  string // sha1sum of separate static dockerinit, if Docker itself was compiled dynamically via ./hack/make.sh dynbinary
	INITPATH  string // custom location to search for a valid dockerinit binary (available for packagers as a last resort escape hatch)
)
