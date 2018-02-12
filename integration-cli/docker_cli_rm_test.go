package main

import (
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestRmContainerOrphaning(c *check.C) {
	dockerfile1 := `FROM busybox:latest
	ENTRYPOINT ["true"]`
	img := "test-container-orphaning"
	dockerfile2 := `FROM busybox:latest
	ENTRYPOINT ["true"]
	MAINTAINER Integration Tests`

	// build first dockerfile
	buildImageSuccessfully(c, img, build.WithDockerfile(dockerfile1))
	img1 := getIDByName(c, img)
	// run container on first image
	dockerCmd(c, "run", img)
	// rebuild dockerfile with a small addition at the end
	buildImageSuccessfully(c, img, build.WithDockerfile(dockerfile2))
	// try to remove the image, should not error out.
	out, _, err := dockerCmdWithError("rmi", img)
	c.Assert(err, check.IsNil, check.Commentf("Expected to removing the image, but failed: %s", out))

	// check if we deleted the first image
	out, _ = dockerCmd(c, "images", "-q", "--no-trunc")
	c.Assert(out, checker.Contains, img1, check.Commentf("Orphaned container (could not find %q in docker images): %s", img1, out))

}
