package main

import (
	"os/exec"
	"strings"

	"github.com/docker/docker/pkg/stringutils"
	"github.com/go-check/check"
)

// tagging a named image in a new unprefixed repo should work
func (s *DockerSuite) TestTagUnprefixedRepoByName(c *check.C) {
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		c.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	tagCmd := exec.Command(dockerBinary, "tag", "busybox:latest", "testfoobarbaz")
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		c.Fatal(out, err)
	}
}

// tagging an image by ID in a new unprefixed repo should work
func (s *DockerSuite) TestTagUnprefixedRepoByID(c *check.C) {
	imageID, err := inspectField("busybox", "Id")
	c.Assert(err, check.IsNil)
	tagCmd := exec.Command(dockerBinary, "tag", imageID, "testfoobarbaz")
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		c.Fatal(out, err)
	}
}

// ensure we don't allow the use of invalid repository names; these tag operations should fail
func (s *DockerSuite) TestTagInvalidUnprefixedRepo(c *check.C) {

	invalidRepos := []string{"fo$z$", "Foo@3cc", "Foo$3", "Foo*3", "Fo^3", "Foo!3", "F)xcz(", "fo%asd"}

	for _, repo := range invalidRepos {
		tagCmd := exec.Command(dockerBinary, "tag", "busybox", repo)
		_, _, err := runCommandWithOutput(tagCmd)
		if err == nil {
			c.Fatalf("tag busybox %v should have failed", repo)
		}
	}
}

// ensure we don't allow the use of invalid tags; these tag operations should fail
func (s *DockerSuite) TestTagInvalidPrefixedRepo(c *check.C) {
	longTag := stringutils.GenerateRandomAlphaOnlyString(121)

	invalidTags := []string{"repo:fo$z$", "repo:Foo@3cc", "repo:Foo$3", "repo:Foo*3", "repo:Fo^3", "repo:Foo!3", "repo:%goodbye", "repo:#hashtagit", "repo:F)xcz(", "repo:-foo", "repo:..", longTag}

	for _, repotag := range invalidTags {
		tagCmd := exec.Command(dockerBinary, "tag", "busybox", repotag)
		_, _, err := runCommandWithOutput(tagCmd)
		if err == nil {
			c.Fatalf("tag busybox %v should have failed", repotag)
		}
	}
}

// ensure we allow the use of valid tags
func (s *DockerSuite) TestTagValidPrefixedRepo(c *check.C) {
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		c.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	validRepos := []string{"fooo/bar", "fooaa/test", "foooo:t"}

	for _, repo := range validRepos {
		tagCmd := exec.Command(dockerBinary, "tag", "busybox:latest", repo)
		_, _, err := runCommandWithOutput(tagCmd)
		if err != nil {
			c.Errorf("tag busybox %v should have worked: %s", repo, err)
			continue
		}
		deleteImages(repo)
	}
}

// tag an image with an existed tag name without -f option should fail
func (s *DockerSuite) TestTagExistedNameWithoutForce(c *check.C) {
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		c.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	tagCmd := exec.Command(dockerBinary, "tag", "busybox:latest", "busybox:test")
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		c.Fatal(out, err)
	}
	tagCmd = exec.Command(dockerBinary, "tag", "busybox:latest", "busybox:test")
	out, _, err := runCommandWithOutput(tagCmd)
	if err == nil || !strings.Contains(out, "Conflict: Tag test is already set to image") {
		c.Fatal("tag busybox busybox:test should have failed,because busybox:test is existed")
	}
}

// tag an image with an existed tag name with -f option should work
func (s *DockerSuite) TestTagExistedNameWithForce(c *check.C) {
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		c.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}

	tagCmd := exec.Command(dockerBinary, "tag", "busybox:latest", "busybox:test")
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		c.Fatal(out, err)
	}
	tagCmd = exec.Command(dockerBinary, "tag", "-f", "busybox:latest", "busybox:test")
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		c.Fatal(out, err)
	}
}

func (s *DockerSuite) TestTagWithSuffixHyphen(c *check.C) {
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		c.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}
	// test repository name begin with '-'
	tagCmd := exec.Command(dockerBinary, "tag", "busybox:latest", "-busybox:test")
	out, _, err := runCommandWithOutput(tagCmd)
	if err == nil || !strings.Contains(out, "Invalid repository name (-busybox). Cannot begin or end with a hyphen") {
		c.Fatal("tag a name begin with '-' should failed")
	}
	// test namespace name begin with '-'
	tagCmd = exec.Command(dockerBinary, "tag", "busybox:latest", "-test/busybox:test")
	out, _, err = runCommandWithOutput(tagCmd)
	if err == nil || !strings.Contains(out, "Invalid namespace name (-test). Cannot begin or end with a hyphen") {
		c.Fatal("tag a name begin with '-' should failed")
	}
	// test index name begin wiht '-'
	tagCmd = exec.Command(dockerBinary, "tag", "busybox:latest", "-index:5000/busybox:test")
	out, _, err = runCommandWithOutput(tagCmd)
	if err == nil || !strings.Contains(out, "Invalid index name (-index:5000). Cannot begin or end with a hyphen") {
		c.Fatal("tag a name begin with '-' should failed")
	}
}

// ensure tagging using official names works
// ensure all tags result in the same name
func (s *DockerSuite) TestTagOfficialNames(c *check.C) {
	names := []string{
		"docker.io/busybox",
		"index.docker.io/busybox",
		"library/busybox",
		"docker.io/library/busybox",
		"index.docker.io/library/busybox",
	}

	for _, name := range names {
		tagCmd := exec.Command(dockerBinary, "tag", "-f", "busybox:latest", name+":latest")
		out, exitCode, err := runCommandWithOutput(tagCmd)
		if err != nil || exitCode != 0 {
			c.Errorf("tag busybox %v should have worked: %s, %s", name, err, out)
			continue
		}

		// ensure we don't have multiple tag names.
		imagesCmd := exec.Command(dockerBinary, "images")
		out, _, err = runCommandWithOutput(imagesCmd)
		if err != nil {
			c.Errorf("listing images failed with errors: %v, %s", err, out)
		} else if strings.Contains(out, name) {
			c.Errorf("images should not have listed '%s'", name)
			deleteImages(name + ":latest")
		}
	}

	for _, name := range names {
		tagCmd := exec.Command(dockerBinary, "tag", "-f", name+":latest", "fooo/bar:latest")
		_, exitCode, err := runCommandWithOutput(tagCmd)
		if err != nil || exitCode != 0 {
			c.Errorf("tag %v fooo/bar should have worked: %s", name, err)
			continue
		}
		deleteImages("fooo/bar:latest")
	}
}
