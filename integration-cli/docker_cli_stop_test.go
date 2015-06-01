package main

import (
	"os/exec"
	"regexp"
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestStopImageLastUsed(c *check.C) {

	var cmd *exec.Cmd
	imageName := "busybox"
	containerName := "testLastUse"
	re := regexp.MustCompile("\"LastUsed\": \"(.+)\"")

	cmd = exec.Command(dockerBinary, "run", "-d", "--name", containerName, imageName, "top")
	runCommand(cmd)

	// Check last used of the image
	cmd = exec.Command(dockerBinary, "inspect", imageName)
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(out, err)
	}
	tBefore, err := time.Parse(time.RFC3339, re.FindAllStringSubmatch(out, -1)[0][1])
	if err != nil {
		c.Fatal(err)
	}

	//sleep some time and then stop to check if was used the image
	time.Sleep(2 * time.Second)
	cmd = exec.Command(dockerBinary, "stop", containerName)
	runCommand(cmd)

	cmd = exec.Command(dockerBinary, "inspect", imageName)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(out, err)
	}
	tAfter, err := time.Parse(time.RFC3339, re.FindAllStringSubmatch(out, -1)[0][1])
	if err != nil {
		c.Fatal(err)
	}

	// Check that the Last used inspect of future is after the inspect of the past
	if !tAfter.After(tBefore) {
		c.Fatalf("Image last used should be in the future: %s > %s", tAfter, tBefore)
	}
}
