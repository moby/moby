package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

// See issue docker/docker#8141
func (s *DockerSuite) TestPullImageWithAliases(c *check.C) {
	defer setupRegistry(c)()

	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	defer deleteImages(repoName)

	repos := []string{}
	for _, tag := range []string{"recent", "fresh"} {
		repos = append(repos, fmt.Sprintf("%v:%v", repoName, tag))
	}

	// Tag and push the same image multiple times.
	for _, repo := range repos {
		if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "tag", "busybox", repo)); err != nil {
			c.Fatalf("Failed to tag image %v: error %v, output %q", repos, err, out)
		}
		defer deleteImages(repo)
		if out, err := exec.Command(dockerBinary, "push", repo).CombinedOutput(); err != nil {
			c.Fatalf("Failed to push image %v: error %v, output %q", repo, err, string(out))
		}
	}

	// Clear local images store.
	args := append([]string{"rmi"}, repos...)
	if out, err := exec.Command(dockerBinary, args...).CombinedOutput(); err != nil {
		c.Fatalf("Failed to clean images: error %v, output %q", err, string(out))
	}

	// Pull a single tag and verify it doesn't bring down all aliases.
	pullCmd := exec.Command(dockerBinary, "pull", repos[0])
	if out, _, err := runCommandWithOutput(pullCmd); err != nil {
		c.Fatalf("Failed to pull %v: error %v, output %q", repoName, err, out)
	}
	if err := exec.Command(dockerBinary, "inspect", repos[0]).Run(); err != nil {
		c.Fatalf("Image %v was not pulled down", repos[0])
	}
	for _, repo := range repos[1:] {
		if err := exec.Command(dockerBinary, "inspect", repo).Run(); err == nil {
			c.Fatalf("Image %v shouldn't have been pulled down", repo)
		}
	}

}

// pulling library/hello-world should show verified message
func (s *DockerSuite) TestPullVerified(c *check.C) {
	c.Skip("Skipping hub dependent test")

	// Image must be pulled from central repository to get verified message
	// unless keychain is manually updated to contain the daemon's sign key.

	verifiedName := "hello-world"
	defer deleteImages(verifiedName)

	// pull it
	expected := "The image you are pulling has been verified"
	pullCmd := exec.Command(dockerBinary, "pull", verifiedName)
	if out, exitCode, err := runCommandWithOutput(pullCmd); err != nil || !strings.Contains(out, expected) {
		if err != nil || exitCode != 0 {
			c.Skip(fmt.Sprintf("pulling the '%s' image from the registry has failed: %v", verifiedName, err))
		}
		c.Fatalf("pulling a verified image failed. expected: %s\ngot: %s, %v", expected, out, err)
	}

	// pull it again
	pullCmd = exec.Command(dockerBinary, "pull", verifiedName)
	if out, exitCode, err := runCommandWithOutput(pullCmd); err != nil || strings.Contains(out, expected) {
		if err != nil || exitCode != 0 {
			c.Skip(fmt.Sprintf("pulling the '%s' image from the registry has failed: %v", verifiedName, err))
		}
		c.Fatalf("pulling a verified image failed. unexpected verify message\ngot: %s, %v", out, err)
	}

}

// pulling an image from the central registry should work
func (s *DockerSuite) TestPullImageFromCentralRegistry(c *check.C) {
	testRequires(c, Network)

	defer deleteImages("hello-world")

	pullCmd := exec.Command(dockerBinary, "pull", "hello-world")
	if out, _, err := runCommandWithOutput(pullCmd); err != nil {
		c.Fatalf("pulling the hello-world image from the registry has failed: %s, %v", out, err)
	}
}

// pulling a non-existing image from the central registry should return a non-zero exit code
func (s *DockerSuite) TestPullNonExistingImage(c *check.C) {
	pullCmd := exec.Command(dockerBinary, "pull", "fooblahblah1234")
	if out, _, err := runCommandWithOutput(pullCmd); err == nil {
		c.Fatalf("expected non-zero exit status when pulling non-existing image: %s", out)
	}
}

// pulling an image from the central registry using official names should work
// ensure all pulls result in the same image
func (s *DockerSuite) TestPullImageOfficialNames(c *check.C) {
	testRequires(c, Network)

	names := []string{
		"docker.io/hello-world",
		"index.docker.io/hello-world",
		"library/hello-world",
		"docker.io/library/hello-world",
		"index.docker.io/library/hello-world",
	}
	for _, name := range names {
		pullCmd := exec.Command(dockerBinary, "pull", name)
		out, exitCode, err := runCommandWithOutput(pullCmd)
		if err != nil || exitCode != 0 {
			c.Errorf("pulling the '%s' image from the registry has failed: %s", name, err)
			continue
		}

		// ensure we don't have multiple image names.
		imagesCmd := exec.Command(dockerBinary, "images")
		out, _, err = runCommandWithOutput(imagesCmd)
		if err != nil {
			c.Errorf("listing images failed with errors: %v", err)
		} else if strings.Contains(out, name) {
			c.Errorf("images should not have listed '%s'", name)
		}
	}
}
