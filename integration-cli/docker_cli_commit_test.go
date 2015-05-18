package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestCommitAfterContainerIsDone(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-a", "stdin", "busybox", "echo", "foo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	waitCmd := exec.Command(dockerBinary, "wait", cleanedContainerID)
	if _, _, err = runCommandWithOutput(waitCmd); err != nil {
		c.Fatalf("error thrown while waiting for container: %s, %v", out, err)
	}

	commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID)
	out, _, err = runCommandWithOutput(commitCmd)
	if err != nil {
		c.Fatalf("failed to commit container to image: %s, %v", out, err)
	}

	cleanedImageID := strings.TrimSpace(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedImageID)
	if out, _, err = runCommandWithOutput(inspectCmd); err != nil {
		c.Fatalf("failed to inspect image: %s, %v", out, err)
	}
}

func (s *DockerSuite) TestCommitWithoutPause(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-a", "stdin", "busybox", "echo", "foo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	waitCmd := exec.Command(dockerBinary, "wait", cleanedContainerID)
	if _, _, err = runCommandWithOutput(waitCmd); err != nil {
		c.Fatalf("error thrown while waiting for container: %s, %v", out, err)
	}

	commitCmd := exec.Command(dockerBinary, "commit", "-p=false", cleanedContainerID)
	out, _, err = runCommandWithOutput(commitCmd)
	if err != nil {
		c.Fatalf("failed to commit container to image: %s, %v", out, err)
	}

	cleanedImageID := strings.TrimSpace(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedImageID)
	if out, _, err = runCommandWithOutput(inspectCmd); err != nil {
		c.Fatalf("failed to inspect image: %s, %v", out, err)
	}
}

//test commit a paused container should not unpause it after commit
func (s *DockerSuite) TestCommitPausedContainer(c *check.C) {
	defer unpauseAllContainers()
	cmd := exec.Command(dockerBinary, "run", "-i", "-d", "busybox")
	out, _, _, err := runCommandWithStdoutStderr(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	cleanedContainerID := strings.TrimSpace(out)
	cmd = exec.Command(dockerBinary, "pause", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(cmd)
	if err != nil {
		c.Fatalf("failed to pause container: %v, output: %q", err, out)
	}

	commitCmd := exec.Command(dockerBinary, "commit", cleanedContainerID)
	out, _, err = runCommandWithOutput(commitCmd)
	if err != nil {
		c.Fatalf("failed to commit container to image: %s, %v", out, err)
	}

	out, err = inspectField(cleanedContainerID, "State.Paused")
	c.Assert(err, check.IsNil)
	if !strings.Contains(out, "true") {
		c.Fatalf("commit should not unpause a paused container")
	}
}

func (s *DockerSuite) TestCommitNewFile(c *check.C) {

	cmd := exec.Command(dockerBinary, "run", "--name", "commiter", "busybox", "/bin/sh", "-c", "echo koye > /foo")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "commit", "commiter")
	imageID, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}
	imageID = strings.Trim(imageID, "\r\n")

	cmd = exec.Command(dockerBinary, "run", imageID, "cat", "/foo")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if actual := strings.Trim(out, "\r\n"); actual != "koye" {
		c.Fatalf("expected output koye received %q", actual)
	}

}

func (s *DockerSuite) TestCommitHardlink(c *check.C) {

	cmd := exec.Command(dockerBinary, "run", "-t", "--name", "hardlinks", "busybox", "sh", "-c", "touch file1 && ln file1 file2 && ls -di file1 file2")
	firstOuput, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}

	chunks := strings.Split(strings.TrimSpace(firstOuput), " ")
	inode := chunks[0]
	found := false
	for _, chunk := range chunks[1:] {
		if chunk == inode {
			found = true
			break
		}
	}
	if !found {
		c.Fatalf("Failed to create hardlink in a container. Expected to find %q in %q", inode, chunks[1:])
	}

	cmd = exec.Command(dockerBinary, "commit", "hardlinks", "hardlinks")
	imageID, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(imageID, err)
	}
	imageID = strings.Trim(imageID, "\r\n")

	cmd = exec.Command(dockerBinary, "run", "-t", "hardlinks", "ls", "-di", "file1", "file2")
	secondOuput, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}

	chunks = strings.Split(strings.TrimSpace(secondOuput), " ")
	inode = chunks[0]
	found = false
	for _, chunk := range chunks[1:] {
		if chunk == inode {
			found = true
			break
		}
	}
	if !found {
		c.Fatalf("Failed to create hardlink in a container. Expected to find %q in %q", inode, chunks[1:])
	}

}

func (s *DockerSuite) TestCommitTTY(c *check.C) {

	cmd := exec.Command(dockerBinary, "run", "-t", "--name", "tty", "busybox", "/bin/ls")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "commit", "tty", "ttytest")
	imageID, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}
	imageID = strings.Trim(imageID, "\r\n")

	cmd = exec.Command(dockerBinary, "run", "ttytest", "/bin/ls")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

}

func (s *DockerSuite) TestCommitWithHostBindMount(c *check.C) {

	cmd := exec.Command(dockerBinary, "run", "--name", "bind-commit", "-v", "/dev/null:/winning", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "commit", "bind-commit", "bindtest")
	imageID, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(imageID, err)
	}

	imageID = strings.Trim(imageID, "\r\n")

	cmd = exec.Command(dockerBinary, "run", "bindtest", "true")

	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

}

func (s *DockerSuite) TestCommitChange(c *check.C) {

	cmd := exec.Command(dockerBinary, "run", "--name", "test", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "commit",
		"--change", "EXPOSE 8080",
		"--change", "ENV DEBUG true",
		"--change", "ENV test 1",
		"--change", "ENV PATH /foo",
		"test", "test-commit")
	imageId, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(imageId, err)
	}
	imageId = strings.Trim(imageId, "\r\n")

	expected := map[string]string{
		"Config.ExposedPorts": "map[8080/tcp:{}]",
		"Config.Env":          "[DEBUG=true test=1 PATH=/foo]",
	}

	for conf, value := range expected {
		res, err := inspectField(imageId, conf)
		c.Assert(err, check.IsNil)
		if res != value {
			c.Errorf("%s('%s'), expected %s", conf, res, value)
		}
	}

}

// TODO: commit --run is deprecated, remove this once --run is removed
func (s *DockerSuite) TestCommitMergeConfigRun(c *check.C) {
	name := "commit-test"
	out, _ := dockerCmd(c, "run", "-d", "-e=FOO=bar", "busybox", "/bin/sh", "-c", "echo testing > /tmp/foo")
	id := strings.TrimSpace(out)

	dockerCmd(c, "commit", `--run={"Cmd": ["cat", "/tmp/foo"]}`, id, "commit-test")

	out, _ = dockerCmd(c, "run", "--name", name, "commit-test")
	if strings.TrimSpace(out) != "testing" {
		c.Fatal("run config in committed container was not merged")
	}

	type cfg struct {
		Env []string
		Cmd []string
	}
	config1 := cfg{}
	if err := inspectFieldAndMarshall(id, "Config", &config1); err != nil {
		c.Fatal(err)
	}
	config2 := cfg{}
	if err := inspectFieldAndMarshall(name, "Config", &config2); err != nil {
		c.Fatal(err)
	}

	// Env has at least PATH loaded as well here, so let's just grab the FOO one
	var env1, env2 string
	for _, e := range config1.Env {
		if strings.HasPrefix(e, "FOO") {
			env1 = e
			break
		}
	}
	for _, e := range config2.Env {
		if strings.HasPrefix(e, "FOO") {
			env2 = e
			break
		}
	}

	if len(config1.Env) != len(config2.Env) || env1 != env2 && env2 != "" {
		c.Fatalf("expected envs to match: %v - %v", config1.Env, config2.Env)
	}

}
