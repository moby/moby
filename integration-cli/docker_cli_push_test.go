package main

import (
	"archive/tar"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/image"
	"github.com/go-check/check"
)

const (
	confirmText  = "want to push to public registry? [y/n]"
	farewellText = "nothing pushed."
	loginText    = "login prior to push:"
)

// Pushing an image to a private registry.
func (s *DockerRegistrySuite) TestPushBusyboxImage(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	// tag the image to upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)
	// push the image to the registry
	dockerCmd(c, "push", repoName)
}

// pushing an image without a prefix should throw an error
func (s *DockerSuite) TestPushUnprefixedRepo(c *check.C) {
	if out, _, err := dockerCmdWithError("push", "busybox"); err == nil {
		c.Fatalf("pushing an unprefixed repo didn't result in a non-zero exit status: %s", out)
	}
}

func (s *DockerRegistrySuite) TestPushUntagged(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	expected := "Repository does not exist"
	if out, _, err := dockerCmdWithError("push", repoName); err == nil {
		c.Fatalf("pushing the image to the private registry should have failed: output %q", out)
	} else if !strings.Contains(out, expected) {
		c.Fatalf("pushing the image failed with an unexpected message: expected %q, got %q", expected, out)
	}
}

func (s *DockerRegistrySuite) TestPushBadTag(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox:latest", privateRegistryURL)

	expected := "does not exist"

	if out, _, err := dockerCmdWithError("push", repoName); err == nil {
		c.Fatalf("pushing the image to the private registry should have failed: output %q", out)
	} else if !strings.Contains(out, expected) {
		c.Fatalf("pushing the image failed with an unexpected message: expected %q, got %q", expected, out)
	}
}

func (s *DockerRegistrySuite) TestPushMultipleTags(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	repoTag1 := fmt.Sprintf("%v/dockercli/busybox:t1", privateRegistryURL)
	repoTag2 := fmt.Sprintf("%v/dockercli/busybox:t2", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoTag1)

	dockerCmd(c, "tag", "busybox", repoTag2)

	dockerCmd(c, "push", repoName)

	// Ensure layer list is equivalent for repoTag1 and repoTag2
	out1, _ := dockerCmd(c, "pull", repoTag1)
	if strings.Contains(out1, "Tag t1 not found") {
		c.Fatalf("Unable to pull pushed image: %s", out1)
	}
	imageAlreadyExists := ": Image already exists"
	var out1Lines []string
	for _, outputLine := range strings.Split(out1, "\n") {
		if strings.Contains(outputLine, imageAlreadyExists) {
			out1Lines = append(out1Lines, outputLine)
		}
	}

	out2, _ := dockerCmd(c, "pull", repoTag2)
	if strings.Contains(out2, "Tag t2 not found") {
		c.Fatalf("Unable to pull pushed image: %s", out1)
	}
	var out2Lines []string
	for _, outputLine := range strings.Split(out2, "\n") {
		if strings.Contains(outputLine, imageAlreadyExists) {
			out1Lines = append(out1Lines, outputLine)
		}
	}

	if len(out1Lines) != len(out2Lines) {
		c.Fatalf("Mismatched output length:\n%s\n%s", out1, out2)
	}

	for i := range out1Lines {
		if out1Lines[i] != out2Lines[i] {
			c.Fatalf("Mismatched output line:\n%s\n%s", out1Lines[i], out2Lines[i])
		}
	}
}

// TestPushBadParentChain tries to push an image with a corrupted parent chain
// in the v1compatibility files, and makes sure the push process fixes it.
func (s *DockerRegistrySuite) TestPushBadParentChain(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/badparent", privateRegistryURL)

	id, err := buildImage(repoName, `
	    FROM busybox
	    CMD echo "adding another layer"
	    `, true)
	if err != nil {
		c.Fatal(err)
	}

	// Push to create v1compatibility file
	dockerCmd(c, "push", repoName)

	// Corrupt the parent in the v1compatibility file from the top layer
	filename := filepath.Join(dockerBasePath, "graph", id, "v1Compatibility")

	jsonBytes, err := ioutil.ReadFile(filename)
	c.Assert(err, check.IsNil, check.Commentf("Could not read v1Compatibility file: %s", err))

	var img image.Image
	err = json.Unmarshal(jsonBytes, &img)
	c.Assert(err, check.IsNil, check.Commentf("Could not unmarshal json: %s", err))

	img.Parent = "1234123412341234123412341234123412341234123412341234123412341234"

	jsonBytes, err = json.Marshal(&img)
	c.Assert(err, check.IsNil, check.Commentf("Could not marshal json: %s", err))

	err = ioutil.WriteFile(filename, jsonBytes, 0600)
	c.Assert(err, check.IsNil, check.Commentf("Could not write v1Compatibility file: %s", err))

	dockerCmd(c, "push", repoName)

	// pull should succeed
	dockerCmd(c, "pull", repoName)
}

func (s *DockerRegistrySuite) TestPushEmptyLayer(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/emptylayer", privateRegistryURL)
	emptyTarball, err := ioutil.TempFile("", "empty_tarball")
	if err != nil {
		c.Fatalf("Unable to create test file: %v", err)
	}
	tw := tar.NewWriter(emptyTarball)
	err = tw.Close()
	if err != nil {
		c.Fatalf("Error creating empty tarball: %v", err)
	}
	freader, err := os.Open(emptyTarball.Name())
	if err != nil {
		c.Fatalf("Could not open test tarball: %v", err)
	}

	importCmd := exec.Command(dockerBinary, "import", "-", repoName)
	importCmd.Stdin = freader
	out, _, err := runCommandWithOutput(importCmd)
	if err != nil {
		c.Errorf("import failed with errors: %v, output: %q", err, out)
	}

	// Now verify we can push it
	if out, _, err := dockerCmdWithError("push", repoName); err != nil {
		c.Fatalf("pushing the image to the private registry has failed: %s, %v", out, err)
	}
}

func (s *DockerTrustSuite) TestTrustedPush(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("Error running trusted push: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithEnvPasswords(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "-f", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithPassphrases(pushCmd, "12345678", "12345678")
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("Error running trusted push: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}
}

// This test ensures backwards compatibility with old ENV variables. Should be
// deprecated by 1.10
func (s *DockerTrustSuite) TestTrustedPushWithDeprecatedEnvPasswords(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusteddeprecated:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithDeprecatedEnvPassphrases(pushCmd, "12345678", "12345678")
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("Error running trusted push: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithFaillingServer(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "-f", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithServer(pushCmd, "https://example.com:81/")
	out, _, err := runCommandWithOutput(pushCmd)
	if err == nil {
		c.Fatalf("Missing error while running trusted push w/ no server")
	}

	if !strings.Contains(string(out), "error contacting notary server") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithoutServerAndUntrusted(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "-f", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", "--disable-content-trust", repoName)
	s.trustedCmdWithServer(pushCmd, "https://example.com/")
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push with no server and --disable-content-trust failed: %s\n%s", err, out)
	}

	if strings.Contains(string(out), "Error establishing connection to notary repository") {
		c.Fatalf("Missing expected output on trusted push with --disable-content-trust:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithExistingTag(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "-f", "busybox", repoName)
	dockerCmd(c, "push", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push with existing tag:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithExistingSignedTag(c *check.C) {
	repoName := fmt.Sprintf("%v/dockerclipushpush/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Do a trusted push
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push with existing tag:\n%s", out)
	}

	// Do another trusted push
	pushCmd = exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err = runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push with existing tag:\n%s", out)
	}

	dockerCmd(c, "rmi", repoName)

	// Try pull to ensure the double push did not break our ability to pull
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	if err != nil {
		c.Fatalf("Error running trusted pull: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Status: Downloaded") {
		c.Fatalf("Missing expected output on trusted pull with --disable-content-trust:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithIncorrectPassphraseForNonRoot(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercliincorretpwd/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Push with default passphrases
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	// Push with wrong passphrases
	pushCmd = exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithPassphrases(pushCmd, "12345678", "87654321")
	out, _, err = runCommandWithOutput(pushCmd)
	if err == nil {
		c.Fatalf("Error missing from trusted push with short targets passphrase: \n%s", out)
	}

	if !strings.Contains(string(out), "password invalid, operation has failed") {
		c.Fatalf("Missing expected output on trusted push with short targets/snapsnot passphrase:\n%s", out)
	}
}

// This test ensures backwards compatibility with old ENV variables. Should be
// deprecated by 1.10
func (s *DockerTrustSuite) TestTrustedPushWithIncorrectDeprecatedPassphraseForNonRoot(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercliincorretdeprecatedpwd/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Push with default passphrases
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	// Push with wrong passphrases
	pushCmd = exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithDeprecatedEnvPassphrases(pushCmd, "12345678", "87654321")
	out, _, err = runCommandWithOutput(pushCmd)
	if err == nil {
		c.Fatalf("Error missing from trusted push with short targets passphrase: \n%s", out)
	}

	if !strings.Contains(string(out), "password invalid, operation has failed") {
		c.Fatalf("Missing expected output on trusted push with short targets/snapsnot passphrase:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithExpiredSnapshot(c *check.C) {
	c.Skip("Currently changes system time, causing instability")
	repoName := fmt.Sprintf("%v/dockercliexpiredsnapshot/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Push with default passphrases
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	// Snapshots last for three years. This should be expired
	fourYearsLater := time.Now().Add(time.Hour * 24 * 365 * 4)

	runAtDifferentDate(fourYearsLater, func() {
		// Push with wrong passphrases
		pushCmd = exec.Command(dockerBinary, "push", repoName)
		s.trustedCmd(pushCmd)
		out, _, err = runCommandWithOutput(pushCmd)
		if err == nil {
			c.Fatalf("Error missing from trusted push with expired snapshot: \n%s", out)
		}

		if !strings.Contains(string(out), "repository out-of-date") {
			c.Fatalf("Missing expected output on trusted push with expired snapshot:\n%s", out)
		}
	})
}

func (s *DockerTrustSuite) TestTrustedPushWithExpiredTimestamp(c *check.C) {
	c.Skip("Currently changes system time, causing instability")
	repoName := fmt.Sprintf("%v/dockercliexpiredtimestamppush/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Push with default passphrases
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	// The timestamps expire in two weeks. Lets check three
	threeWeeksLater := time.Now().Add(time.Hour * 24 * 21)

	// Should succeed because the server transparently re-signs one
	runAtDifferentDate(threeWeeksLater, func() {
		pushCmd := exec.Command(dockerBinary, "push", repoName)
		s.trustedCmd(pushCmd)
		out, _, err := runCommandWithOutput(pushCmd)
		if err != nil {
			c.Fatalf("Error running trusted push: %s\n%s", err, out)
		}
		if !strings.Contains(string(out), "Signing and pushing trust metadata") {
			c.Fatalf("Missing expected output on trusted push with expired timestamp:\n%s", out)
		}
	})
}

func (s *DockerSuite) TestPushOfficialImage(c *check.C) {
	var reErr = regexp.MustCompile(`rename your repository to[^:]*:\s*<user>/busybox\b`)

	// push busybox to public registry as "library/busybox"
	cmd := exec.Command(dockerBinary, "push", "library/busybox")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.Fatalf("Failed to get stdout pipe for process: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		c.Fatalf("Failed to get stderr pipe for process: %v", err)
	}
	if err := cmd.Start(); err != nil {
		c.Fatalf("Failed to start pushing to public registry: %v", err)
	}
	outReader := bufio.NewReader(stdout)
	errReader := bufio.NewReader(stderr)
	line, isPrefix, err := errReader.ReadLine()
	if err != nil {
		c.Fatalf("Failed to read farewell: %v", err)
	}
	if isPrefix {
		c.Errorf("Got unexpectedly long output.")
	}
	if !reErr.Match(line) {
		c.Errorf("Got unexpected output %q", line)
	}
	if line, _, err = outReader.ReadLine(); err != io.EOF {
		c.Errorf("Expected EOF, not: %q", line)
	}
	for ; err != io.EOF; line, _, err = errReader.ReadLine() {
		c.Errorf("Expected no message on stderr, got: %q", string(line))
	}

	// Wait for command to finish with short timeout.
	finish := make(chan struct{})
	go func() {
		if err := cmd.Wait(); err == nil {
			c.Error("Push command should have failed.")
		}
		close(finish)
	}()
	select {
	case <-finish:
	case <-time.After(1 * time.Second):
		c.Fatalf("Docker push failed to exit.")
	}
}

func readConfirmText(c *check.C, out *bufio.Reader) {
	done := make(chan struct{})
	go func() {
		line, err := out.ReadBytes(']')
		if err != nil {
			c.Fatalf("Failed to read a confirmation text for a push: %v, out: %s", err, line)
		}
		if !strings.HasSuffix(strings.ToLower(string(line)), confirmText) {
			c.Fatalf("Expected confirmation text %q, not: %q", confirmText, line)
		}
		buf := make([]byte, 4)
		n, err := out.Read(buf)
		if err != nil {
			c.Fatalf("Failed to read confirmation text for a push: %v", err)
		}
		if n > 2 || n < 1 || buf[0] != ':' {
			c.Fatalf("Got unexpected line ending: %q", string(buf))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		c.Fatalf("Timeout while waiting on confirmation text.")
	}
}

func (s *DockerSuite) TestPushToPublicRegistry(c *check.C) {
	repoName := "docker.io/dockercli/busybox"
	// tag the image to upload it to the private registry
	tagCmd := exec.Command(dockerBinary, "tag", "busybox", repoName)
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		c.Fatalf("image tagging failed: %s, %v", out, err)
	}
	defer deleteImages(repoName)

	// `sayNo` says whether to terminate communication with negative answer or
	// by closing input stream
	runTest := func(pushCmd *exec.Cmd, sayNo bool) {
		stdin, err := pushCmd.StdinPipe()
		if err != nil {
			c.Fatalf("Failed to get stdin pipe for process: %v", err)
		}
		stdout, err := pushCmd.StdoutPipe()
		if err != nil {
			c.Fatalf("Failed to get stdout pipe for process: %v", err)
		}
		stderr, err := pushCmd.StderrPipe()
		if err != nil {
			c.Fatalf("Failed to get stderr pipe for process: %v", err)
		}
		if err := pushCmd.Start(); err != nil {
			c.Fatalf("Failed to start pushing to private registry: %v", err)
		}

		outReader := bufio.NewReader(stdout)

		readConfirmText(c, outReader)

		stdin.Write([]byte("\n"))
		readConfirmText(c, outReader)
		stdin.Write([]byte("  \n"))
		readConfirmText(c, outReader)
		stdin.Write([]byte("foo\n"))
		readConfirmText(c, outReader)
		stdin.Write([]byte("no\n"))
		readConfirmText(c, outReader)
		if sayNo {
			stdin.Write([]byte(" n \n"))
		} else {
			stdin.Close()
		}

		line, isPrefix, err := outReader.ReadLine()
		if err != nil {
			c.Fatalf("Failed to read farewell: %v", err)
		}
		if isPrefix {
			c.Errorf("Got unexpectedly long output.")
		}
		lowered := strings.ToLower(string(line))
		if sayNo {
			if !strings.HasSuffix(lowered, farewellText) {
				c.Errorf("Expected farewell %q, not: %q", farewellText, string(line))
			}
			if strings.Contains(lowered, confirmText) {
				c.Errorf("God unexpected confirmation text: %q", string(line))
			}
		} else {
			if lowered != "eof" {
				c.Errorf("Expected \"EOF\" not: %q", string(line))
			}
			if line, _, err = outReader.ReadLine(); err != io.EOF {
				c.Errorf("Expected EOF, not: %q", line)
			}
		}
		if line, _, err = outReader.ReadLine(); err != io.EOF {
			c.Errorf("Expected EOF, not: %q", line)
		}
		errReader := bufio.NewReader(stderr)
		for ; err != io.EOF; line, _, err = errReader.ReadLine() {
			c.Errorf("Expected no message on stderr, got: %q", string(line))
		}

		// Wait for command to finish with short timeout.
		finish := make(chan struct{})
		go func() {
			if err := pushCmd.Wait(); err != nil && sayNo {
				c.Error(err)
			} else if err == nil && !sayNo {
				c.Errorf("Process should have failed after closing input stream.")
			}
			close(finish)
		}()
		select {
		case <-finish:
		case <-time.After(500 * time.Millisecond):
			cause := "standard input close"
			if sayNo {
				cause = "negative answer"
			}
			c.Fatalf("Docker push failed to exit on %s.", cause)
		}
	}
	runTest(exec.Command(dockerBinary, "push", repoName), false)
	runTest(exec.Command(dockerBinary, "push", repoName), true)
}

func (s *DockerDaemonSuite) TestPushToPublicRegistryNoConfirm(c *check.C) {
	daemonArgs := []string{"--confirm-def-push=false"}
	if err := s.d.StartWithBusybox(daemonArgs...); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing { %s } flags: %v", strings.Join(daemonArgs, ", "), err)
	}

	repoName := "docker.io/user/hello-world"
	if out, err := s.d.Cmd("tag", "busybox", repoName); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}

	runTest := func(name string, arg ...string) {
		args := []string{"--host", s.d.sock(), name}
		args = append(args, arg...)
		c.Logf("Running %s %s %s", dockerBinary, name, strings.Join(args, " "))
		cmd := exec.Command(dockerBinary, args...)

		stdin, err := cmd.StdinPipe()
		if err != nil {
			c.Fatalf("Failed to get stdin pipe for process: %v", err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			c.Fatalf("Failed to get stdout pipe for process: %v", err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			c.Fatalf("Failed to get stderr pipe for process: %v", err)
		}
		if err := cmd.Start(); err != nil {
			c.Fatalf("Failed to start pushing to private registry: %v", err)
		}
		outReader := bufio.NewReader(stdout)

		go io.Copy(os.Stderr, stderr)

		errChan := make(chan error)
		go func() {
			for {
				line, err := outReader.ReadBytes('\n')
				if err != nil {
					if err == io.EOF {
						errChan <- nil
					} else {
						errChan <- fmt.Errorf("Failed to read line: %v", err)
					}
					break
				}
				c.Logf("output of push command: %q", line)
				trimmed := strings.ToLower(strings.TrimSpace(string(line)))
				if strings.HasSuffix(trimmed, confirmText) {
					errChan <- fmt.Errorf("Got unexpected confirmation text: %q", line)
					break
				}
				if strings.HasSuffix(trimmed, loginText) {
					errChan <- nil
					break
				}
			}
		}()
		select {
		case err := <-errChan:
			if err != nil {
				c.Fatal(err.Error())
			}
		case <-time.After(10 * time.Second):
			c.Fatal("Push command timeouted!")
		}
		stdin.Close()

		// Wait for command to finish with short timeout.
		finish := make(chan struct{})
		go func() {
			if err := cmd.Wait(); err == nil {
				c.Errorf("Process should have failed after closing input stream.")
			}
			close(finish)
		}()
		select {
		case <-finish:
		case <-time.After(500 * time.Millisecond):
			c.Fatalf("Docker push failed to exit!")
		}
	}

	runTest("push", repoName)
	runTest("push", "-f", repoName)
}

func (s *DockerRegistrySuite) TestPushToAdditionalRegistry(c *check.C) {
	if err := s.d.StartWithBusybox("--add-registry=" + s.reg.url); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing add-registry=%s: %v", s.reg.url, err)
	}

	bbImg := s.d.getAndTestImageEntry(c, 1, "busybox", "")

	// push busybox to additional registry as "library/busybox" and remove all local images
	if out, err := s.d.Cmd("tag", "busybox", "library/busybox"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("push", "library/busybox"); err != nil {
		c.Fatalf("failed to push image library/busybox: error %v, output %q", err, out)
	}
	toRemove := []string{"busybox", "library/busybox"}
	if out, err := s.d.Cmd("rmi", toRemove...); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	s.d.getAndTestImageEntry(c, 0, "", "")

	// pull it from additional registry
	if _, err := s.d.Cmd("pull", "library/busybox"); err != nil {
		c.Fatalf("we should have been able to pull library/busybox from %q: %v", s.reg.url, err)
	}
	bb2Img := s.d.getAndTestImageEntry(c, 1, s.reg.url+"/library/busybox", "")
	if bb2Img.size != bbImg.size {
		c.Fatalf("expected %s and %s to have the same size (%s != %s)", bb2Img.name, bbImg.name, bb2Img.size, bbImg.size)
	}
}

func (s *DockerRegistrySuite) TestPushCustomTagToAdditionalRegistry(c *check.C) {
	if err := s.d.StartWithBusybox("--add-registry=" + s.reg.url); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing add-registry=%s: %v", s.reg.url, err)
	}

	busyboxID := s.d.getAndTestImageEntry(c, 1, "busybox", "").id

	if out, err := s.d.Cmd("tag", "busybox", "user/busybox:1.2.3"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("tag", "busybox", s.reg.url+"/user/busybox:latest"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("push", "user/busybox:1.2.3"); err != nil {
		c.Fatalf("failed to push image user/busybox: error %v, output %q", err, out)
	}
	s.d.getAndTestImageEntry(c, 3, "user/busybox", busyboxID)
	toRemove := []string{"user/busybox:1.2.3"}
	if out, err := s.d.Cmd("rmi", toRemove...); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	s.d.getAndTestImageEntry(c, 2, s.reg.url+"/user/busybox", busyboxID)
}
