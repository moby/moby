package main

import (
	"fmt"
	"os/exec"
	"regexp"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestUpdateDryRun(c *check.C) {
	RunUpdateTest(s, c, true)
}

func (s *DockerSuite) TestUpdateWithPull(c *check.C) {
	RunUpdateTest(s, c, false)
}

func RunUpdateTest(s *DockerSuite, c *check.C, dry_run bool) {
	defer setupRegistry(c)()

	name := "testupdateimage"
	repoName := fmt.Sprintf("%v/dockercli/%s", privateRegistryURL, name)

	oldName := repoName + ":old"

	defer deleteImages(repoName, oldName)

	//build version 1 ("old" version)
	_, err := buildImage(oldName, `FROM busybox
RUN echo "A"`, true)

	if err != nil {
		c.Fatal("Error building image", err)
	}

	//build version 2 ("latest" version)
	_, err = buildImage(repoName, `FROM busybox
RUN echo "A"
RUN echo "B"`, true)

	if err != nil {
		c.Fatal("Error building image", err)
	}

	//push the latest version to repo, and delete local copy
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "push", repoName)); err != nil {
		c.Fatalf("pushing the image to the private registry has failed: %s, %v", out, err)
	}

	deleteImages(repoName)

	//mark the old version as latest
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "tag", oldName, repoName)); err != nil {
		c.Fatalf("Failed to tag image %v: error %v, output %q", repoName, err, out)
	}

	//now do the update
	if dry_run {
		out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "update", "--dry-run"))
		if err != nil {
			c.Fatalf("update command failed: %s, %v", out, err)
		}
	} else {
		out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "update"))
		if err != nil {
			c.Fatalf("update command failed: %s, %v", out, err)
		}
	}

	//get the list of local images and check wheter update had any effect
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "images"))
	if err != nil {
		c.Fatalf("Failed to get list of images: %s, %v", out, err)
	}

	match1 := regexp.MustCompile("dockercli/testupdateimage\\s+latest\\s+([a-z0-9]+)\\s").FindAllStringSubmatch(out, -1)
	match2 := regexp.MustCompile("dockercli/testupdateimage\\s+old\\s+([a-z0-9]+)\\s").FindAllStringSubmatch(out, -1)

	if len(match1) < 1 || len(match2) < 1 || len(match1[0]) < 2 || len(match2[0]) < 2 {
		c.Fatalf("Images to update are not present in the list")
	}

	if dry_run {
		if match1[0][1] != match2[0][1] {
			//the latest should be the same as old (since we didn't pull)
			c.Fatalf("Update ignored --dry-run paramater")
		}
	} else {
		if match1[0][1] == match2[0][1] {
			//the latest should be different from the old (since we pulled)
			c.Fatalf("Update didn't pull the latest version")
		}
	}
}
