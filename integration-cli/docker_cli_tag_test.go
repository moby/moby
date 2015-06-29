package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func (s *DockerSuite) TestTagWithPrefixHyphen(c *check.C) {
	if err := pullImageIfNotExist("busybox:latest"); err != nil {
		c.Fatal("couldn't find the busybox:latest image locally and failed to pull it")
	}
	// test repository name begin with '-'
	tagCmd := exec.Command(dockerBinary, "tag", "busybox:latest", "-busybox:test")
	out, _, err := runCommandWithOutput(tagCmd)
	if err == nil || !strings.Contains(out, "repository name component must match") {
		c.Fatal("tag a name begin with '-' should failed")
	}
	// test namespace name begin with '-'
	tagCmd = exec.Command(dockerBinary, "tag", "busybox:latest", "-test/busybox:test")
	out, _, err = runCommandWithOutput(tagCmd)
	if err == nil || !strings.Contains(out, "repository name component must match") {
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

func tagLinesEqual(expected, actual []string, allowEmptyImageID bool) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if i == 2 && actual[i] == "" && allowEmptyImageID {
			continue
		}
		if expected[i] != actual[i] {
			return false
		}
	}
	return true
}

func assertTagListEqual(c *check.C, d *Daemon, allowEmptyImageID bool, names []string, expectedString string) {
	var (
		reLine = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\w+)?`)
		reLog  = regexp.MustCompile(`(DEBU|WARN|ERR|INFO|FATA)|(level=(warn|info|err|fata|debu))`)
	)
	out, err := d.Cmd("tag", append([]string{"-l"}, names...)...)
	if err != nil {
		c.Fatalf("Failed to list remote tags for %s: %v", strings.Join(names, " "), err)
	}
	parseString := func(str string, nLinesToSkip int) [][]string {
		res := [][]string{}
		i := 0
		for _, line := range strings.Split(str, "\n") {
			if reLog.MatchString(line) {
				c.Logf("%s", line)
				continue
			}
			if i < nLinesToSkip || line == "" {
				i += 1
				continue
			}
			i += 1
			match := reLine.FindStringSubmatch(line)
			if len(match) == 0 {
				c.Errorf("Failed to parse line %q", line)
				continue
			}
			res = append(res, match[1:])
		}
		return res
	}
	actual := parseString(out, 1)
	expected := parseString(expectedString, 0)
	if len(actual) != len(expected) {
		c.Errorf("Got unexpected number of results (%d), expected was %d.", len(actual), len(expected))
		c.Logf("Expected lines:")
		for i, strs := range expected {
			c.Logf("		#%3d: %s", i, strings.Join(strs, "\t"))
		}
		c.Logf("Actual lines:")
		for i, strs := range actual {
			c.Logf("		#%3d: %s", i, strings.Join(strs, "\t"))
		}
		//time.Sleep(time.Minute * 10000)
	} else {
		errorReported := false
		for i := range actual {
			if !tagLinesEqual(expected[i], actual[i], allowEmptyImageID) {
				if !errorReported {
					c.Errorf("Expected line #%3d: %s", i, strings.Join(expected[i], "\t"))
					//time.Sleep(time.Minute * 10000)
					errorReported = true
				} else {
					c.Logf("Expected line #%3d: %s", i, strings.Join(expected[i], "\t"))
				}
				c.Logf("Actual line #%3d  : %s", i, strings.Join(actual[i], "\t"))
			}
		}
	}
}

func (s *DockerRegistriesSuite) TestTagListRemoteRepository(c *check.C) {
	d := NewDaemon(c)
	daemonArgs := []string{"--add-registry=" + s.reg2.url}
	if err := d.StartWithBusybox(daemonArgs...); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing { %s } flags: %v", strings.Join(daemonArgs, ", "), err)
	}
	defer d.Stop()

	{ // load hello-world
		bb := filepath.Join(d.folder, "hello-world.tar")
		if _, err := os.Stat(bb); err != nil {
			if !os.IsNotExist(err) {
				c.Fatalf("unexpected error on hello-world.tar stat: %v", err)
			}
			// saving busybox image from main daemon
			if err := exec.Command(dockerBinary, "save", "--output", bb, "hello-world:frozen").Run(); err != nil {
				c.Fatalf("could not save hello-world:frozen image to %q: %v", bb, err)
			}
		}
		// loading hello-world image to this daemon
		if _, err := d.Cmd("load", "--input", bb); err != nil {
			c.Fatalf("could not load hello-world image: %v", err)
		}
		if err := os.Remove(bb); err != nil {
			d.c.Logf("could not remove %s: %v", bb, err)
		}
	}
	busyboxID := d.getAndTestImageEntry(c, 2, "busybox", "").id
	helloWorldID := d.getAndTestImageEntry(c, 2, "hello-world", "").id

	for _, tag := range []string{"1.1-busy", "1.2-busy", "1.3-busy"} {
		dest := s.reg1.url + "/foo/busybox:" + tag
		if out, err := d.Cmd("tag", "busybox", dest); err != nil {
			c.Fatalf("failed to tag image %q as %q: error %v, output %q", "busybox", dest, err, out)
		}
	}
	for _, tag := range []string{"1.4-hell", "1.5-hell"} {
		dest := s.reg1.url + "/foo/busybox:" + tag
		if out, err := d.Cmd("tag", "hello-world:frozen", dest); err != nil {
			c.Fatalf("failed to tag image %q as %q: error %v, output %q", "busybox", dest, err, out)
		}
	}
	for _, tag := range []string{"2.1-busy", "2.2-busy", "2.3-busy"} {
		dest := s.reg2.url + "/foo/busybox:" + tag
		if out, err := d.Cmd("tag", "busybox", dest); err != nil {
			c.Fatalf("failed to tag image %q as %q: error %v, output %q", "busybox", dest, err, out)
		}
	}
	for _, tag := range []string{"2.4-hell", "2.5-hell"} {
		dest := s.reg2.url + "/foo/busybox:" + tag
		if out, err := d.Cmd("tag", "hello-world:frozen", dest); err != nil {
			c.Fatalf("failed to tag image %q as %q: error %v, output %q", "busybox", dest, err, out)
		}
	}
	localTags := []string{}
	for ri, reg := range []*testRegistryV2{s.reg1, s.reg2} {
		for i := 0; i < 5; i++ {
			imgNames := []string{"busy", "hell"}
			if (ri+i)%3 == 0 {
				continue // upload 2/3 of registries
			}
			tag := fmt.Sprintf("%s/foo/busybox:%d.%d-%s", reg.url, ri+1, i+1, imgNames[i/3])
			if out, err := d.Cmd("push", tag); err != nil {
				c.Fatalf("push of %q should have succeeded: %v, output: %s", tag, err, out)
			}
			localTags = append(localTags, tag)
		}
	}

	assertTagListEqual(c, d, true, []string{s.reg1.url + "/foo/busybox"},
		fmt.Sprintf("%s/foo/busybox		1.2-busy		%s\n", s.reg1.url, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		1.3-busy		%s\n", s.reg1.url, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		1.5-hell		%s\n", s.reg1.url, helloWorldID))

	assertTagListEqual(c, d, true, []string{s.reg2.url + "/foo/busybox"},
		fmt.Sprintf("%s/foo/busybox		2.1-busy		%s\n", s.reg2.url, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		2.2-busy		%s\n", s.reg2.url, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		2.4-hell		%s\n", s.reg2.url, helloWorldID)+
			fmt.Sprintf("%s/foo/busybox		2.5-hell		%s\n", s.reg2.url, helloWorldID))

	assertTagListEqual(c, d, true, []string{"foo/busybox"},
		fmt.Sprintf("%s/foo/busybox		2.1-busy		%s\n", s.reg2.url, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		2.2-busy		%s\n", s.reg2.url, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		2.4-hell		%s\n", s.reg2.url, helloWorldID)+
			fmt.Sprintf("%s/foo/busybox		2.5-hell		%s\n", s.reg2.url, helloWorldID))

	// now delete all local images
	if out, err := d.Cmd("rmi", localTags...); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", localTags, err, out)
	}

	// and try again
	assertTagListEqual(c, d, true, []string{"foo/busybox", s.reg1.url + "/foo/busybox"},
		fmt.Sprintf("%s/foo/busybox		2.1-busy		%s\n", s.reg2.url, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		2.2-busy		%s\n", s.reg2.url, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		2.4-hell		%s\n", s.reg2.url, helloWorldID)+
			fmt.Sprintf("%s/foo/busybox		2.5-hell		%s\n", s.reg2.url, helloWorldID)+
			fmt.Sprintf("%s/foo/busybox		1.2-busy		%s\n", s.reg1.url, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		1.3-busy		%s\n", s.reg1.url, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		1.5-hell		%s\n", s.reg1.url, helloWorldID))

	// skip over bad repositories
	assertTagListEqual(c, d, true, []string{"foo/.us&box", "notexistent/busybox", s.reg1.url + "/foo/busybox"},
		fmt.Sprintf("%s/foo/busybox		1.2-busy		%s\n", s.reg1.url, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		1.3-busy		%s\n", s.reg1.url, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		1.5-hell		%s\n", s.reg1.url, helloWorldID))
}
