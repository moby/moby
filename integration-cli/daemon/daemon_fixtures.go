package daemon

import (
	"github.com/docker/docker/integration-cli/fixtures/load"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (d *Daemon) EnsureFrozenImagesLinux(c *check.C) {
	// FIXME(vdemeester) this is duplicated with fixtures_linx_daemon, should go away
	images := []string{"busybox:latest", "hello-world:frozen", "debian:jessie"}
	err := load.FrozenImagesLinux(d.dockerBinary, d.PrependHostArg([]string{}), images...)
	c.Assert(err, checker.IsNil, check.Commentf("Couldn't load frozen image"))
	for _, img := range images {
		d.protectedImages[img] = struct{}{}
	}
}
