package main

import (
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/go-check/check"
)

// tagging a named image in a new unprefixed repo should work
func (s *DockerSuite) TestTagUnprefixedRepoByName(c *check.C) {
	testRequires(c, DaemonIsLinux)
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		c.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	dockerCmd(c, "tag", "busybox:latest", "testfoobarbaz")
}

// tagging an image by ID in a new unprefixed repo should work
func (s *DockerSuite) TestTagUnprefixedRepoByID(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imageID, err := inspectField("busybox", "Id")
	c.Assert(err, check.IsNil)
	dockerCmd(c, "tag", imageID, "testfoobarbaz")
}

// ensure we don't allow the use of invalid repository names; these tag operations should fail
func (s *DockerSuite) TestTagInvalidUnprefixedRepo(c *check.C) {
	invalidRepos := []string{"fo$z$", "Foo@3cc", "Foo$3", "Foo*3", "Fo^3", "Foo!3", "F)xcz(", "fo%asd"}

	for _, repo := range invalidRepos {
		out, _, err := dockerCmdWithError("tag", "busybox", repo)
		c.Assert(err, checker.NotNil, check.Commentf("tag busybox %v should have failed : %v", repo, out))
	}
}

// ensure we don't allow the use of invalid tags; these tag operations should fail
func (s *DockerSuite) TestTagInvalidPrefixedRepo(c *check.C) {
	longTag := stringutils.GenerateRandomAlphaOnlyString(121)

	invalidTags := []string{"repo:fo$z$", "repo:Foo@3cc", "repo:Foo$3", "repo:Foo*3", "repo:Fo^3", "repo:Foo!3", "repo:%goodbye", "repo:#hashtagit", "repo:F)xcz(", "repo:-foo", "repo:..", longTag}

	for _, repotag := range invalidTags {
		out, _, err := dockerCmdWithError("tag", "busybox", repotag)
		c.Assert(err, checker.NotNil, check.Commentf("tag busybox %v should have failed : %v", repotag, out))
	}
}

// ensure we allow the use of valid tags
func (s *DockerSuite) TestTagValidPrefixedRepo(c *check.C) {
	testRequires(c, DaemonIsLinux)
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		c.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	validRepos := []string{"fooo/bar", "fooaa/test", "foooo:t"}

	for _, repo := range validRepos {
		_, _, err := dockerCmdWithError("tag", "busybox:latest", repo)
		if err != nil {
			c.Errorf("tag busybox %v should have worked: %s", repo, err)
			continue
		}
		deleteImages(repo)
	}
}

// tag an image with an existed tag name without -f option should fail
func (s *DockerSuite) TestTagExistedNameWithoutForce(c *check.C) {
	testRequires(c, DaemonIsLinux)
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		c.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	dockerCmd(c, "tag", "busybox:latest", "busybox:test")
	out, _, err := dockerCmdWithError("tag", "busybox:latest", "busybox:test")

	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "Conflict: Tag busybox:test is already set to image", check.Commentf("tag busybox busybox:test should have failed,because busybox:test is existed"))
}

// tag an image with an existed tag name with -f option should work
func (s *DockerSuite) TestTagExistedNameWithForce(c *check.C) {
	testRequires(c, DaemonIsLinux)
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		c.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	dockerCmd(c, "tag", "busybox:latest", "busybox:test")
	dockerCmd(c, "tag", "-f", "busybox:latest", "busybox:test")
}

func (s *DockerSuite) TestTagWithPrefixHyphen(c *check.C) {
	testRequires(c, DaemonIsLinux)
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		c.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	// test repository name begin with '-'
	out, _, err := dockerCmdWithError("tag", "busybox:latest", "-busybox:test")
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "repository name component must match", check.Commentf("tag a name begin with '-' should failed"))

	// test namespace name begin with '-'
	out, _, err = dockerCmdWithError("tag", "busybox:latest", "-test/busybox:test")
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "repository name component must match", check.Commentf("tag a name begin with '-' should failed"))

	// test index name begin with '-'
	out, _, err = dockerCmdWithError("tag", "busybox:latest", "-index:5000/busybox:test")
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "Invalid index name (-index:5000). Cannot begin or end with a hyphen", check.Commentf("tag a name begin with '-' should failed"))
}

// ensure tagging using official names works
// ensure all tags result in the same name
func (s *DockerSuite) TestTagOfficialNames(c *check.C) {
	testRequires(c, DaemonIsLinux)
	names := []string{
		"docker.io/busybox",
		"index.docker.io/busybox",
		"library/busybox",
		"docker.io/library/busybox",
		"index.docker.io/library/busybox",
	}

	for _, name := range names {
		out, exitCode, err := dockerCmdWithError("tag", "-f", "busybox:latest", name+":latest")
		if err != nil || exitCode != 0 {
			c.Errorf("tag busybox %v should have worked: %s, %s", name, err, out)
			continue
		}

		// ensure we don't have multiple tag names.
		out, _, err = dockerCmdWithError("images")
		if err != nil {
			c.Errorf("listing images failed with errors: %v, %s", err, out)
		} else if strings.Contains(out, name) {
			c.Errorf("images should not have listed '%s'", name)
			deleteImages(name + ":latest")
		}
	}

	for _, name := range names {
		_, exitCode, err := dockerCmdWithError("tag", "-f", name+":latest", "fooo/bar:latest")
		if err != nil || exitCode != 0 {
			c.Errorf("tag %v fooo/bar should have worked: %s", name, err)
			continue
		}
		deleteImages("fooo/bar:latest")
	}
}

// ensure tags can not match digests
func (s *DockerSuite) TestTagMatchesDigest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		c.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}
	digest := "busybox@sha256:abcdef76720241213f5303bda7704ec4c2ef75613173910a56fb1b6e20251507"
	// test setting tag fails
	_, _, err := dockerCmdWithError("tag", "-f", "busybox:latest", digest)
	if err == nil {
		c.Fatal("digest tag a name should have failed")
	}
	// check that no new image matches the digest
	_, _, err = dockerCmdWithError("inspect", digest)
	if err == nil {
		c.Fatal("inspecting by digest should have failed")
	}
}
