package main

import (
	"fmt"
	"os/exec"
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

func assertTagListEqual(c *check.C, remote, allowEmptyImageID bool, names []string, expectedString string) {
	var (
		reLine = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\w+)?`)
		reLog  = regexp.MustCompile(`(DEBU|WARN|ERR|INFO|FATA)|(level=(warn|info|err|fata|debu))`)
	)
	args := []string{"tag", "-l"}
	if remote {
		args = append(args, "-r")
	}
	args = append(args, names...)
	cmd := exec.Command(dockerBinary, args...)
	out, _, err := runCommandWithOutput(cmd)
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
	} else {
		errorReported := false
		for i := range actual {
			if !tagLinesEqual(expected[i], actual[i], allowEmptyImageID) {
				if !errorReported {
					c.Errorf("Expected line #%-3d: %s", i, strings.Join(expected[i], "\t"))
					errorReported = true
				} else {
					c.Logf("Expected line #%-3d: %s", i, strings.Join(expected[i], "\t"))
				}
				c.Logf("Actual line #%-3d  : %s", i, strings.Join(actual[i], "\t"))
			}
		}
	}
}

func (s *DockerRegistrySuite) TestTagListRemoteRepository(c *check.C) {
	getImageIdOf := func(imageName string) string {
		cmd := exec.Command(dockerBinary, "images", "-q", "--no-trunc", imageName)
		out, _, err := runCommandWithOutput(cmd)
		if err != nil {
			c.Fatalf("failed to get image id for %q: %v", imageName, err)
		}
		ids := strings.Split(out, "\n")
		if len(ids) < 1 {
			c.Fatalf("image %q not found", imageName)
		}
		return ids[0]
	}
	busyboxID := getImageIdOf("busybox")
	helloWorldID := getImageIdOf("hello-world")

	/* Create tags:
	 * - <private>/foo/busybox:1-busy     B
	 * - <private>/foo/busybox:2-busy   U B
	 * - <private>/foo/busybox:3-busy   U B
	 * - <private>/foo/busybox:4-hell     H
	 * - <private>/foo/busybox:5-hell   U H
	 * - <private>/bar/busybox:6-hell   U H
	 * - <private>/bar/busybox:7-busy     B
	 * U - upload this tag to private registry
	 * B - tag points to local busybox image
	 * H - tag points to local hello-world image
	 */
	namespaces := []string{"foo", "bar"}
	imgNames := []string{"busybox", "hello-world"}
	imgTags := []string{"busy", "hell"}
	localTags := []string{}
	for i := 0; i < 7; i++ {
		src := []string{imgNames[0], imgNames[1] + ":" + "frozen"}[(i/3)%2]
		dest := fmt.Sprintf("%s/%s/busybox:%d-%s", privateRegistryURL, namespaces[i/5], i+1, imgTags[(i/3)%2])
		cmd := exec.Command(dockerBinary, "tag", src, dest)
		if out, _, err := runCommandWithOutput(cmd); err != nil {
			c.Fatalf("failed to tag image %q as %q: error %v, output %q", src, dest, err, out)
		}
		localTags = append(localTags, dest)
		if i%3 == 0 {
			continue // push 2/3 of all tags
		}
		cmd = exec.Command(dockerBinary, "push", dest)
		if out, _, err := runCommandWithOutput(cmd); err != nil {
			c.Fatalf("push of %q should have succeeded: %v, output: %s", dest, err, out)
		}
	}

	// list remote tags
	assertTagListEqual(c, true, true, []string{privateRegistryURL + "/foo/busybox"},
		fmt.Sprintf("%s/foo/busybox		2-busy		%s\n", privateRegistryURL, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		3-busy		%s\n", privateRegistryURL, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		5-hell		%s\n", privateRegistryURL, helloWorldID))

	// list local tags
	assertTagListEqual(c, false, true, []string{privateRegistryURL + "/foo/busybox"},
		fmt.Sprintf("%s/foo/busybox		1-busy		%s\n", privateRegistryURL, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		2-busy		%s\n", privateRegistryURL, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		3-busy		%s\n", privateRegistryURL, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		4-hell		%s\n", privateRegistryURL, helloWorldID)+
			fmt.Sprintf("%s/foo/busybox		5-hell		%s\n", privateRegistryURL, helloWorldID))

	deleteImages(localTags...)

	// and try to list remote tags again
	assertTagListEqual(c, true, true, []string{privateRegistryURL + "/foo/busybox"},
		fmt.Sprintf("%s/foo/busybox		2-busy		%s\n", privateRegistryURL, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		3-busy		%s\n", privateRegistryURL, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		5-hell		%s\n", privateRegistryURL, helloWorldID))

	// and now local ones - this should fallback to remote query
	assertTagListEqual(c, false, true, []string{privateRegistryURL + "/foo/busybox"},
		fmt.Sprintf("%s/foo/busybox		2-busy		%s\n", privateRegistryURL, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		3-busy		%s\n", privateRegistryURL, busyboxID)+
			fmt.Sprintf("%s/foo/busybox		5-hell		%s\n", privateRegistryURL, helloWorldID))

	// skip over bad repositories
	assertTagListEqual(c, true, true, []string{
		privateRegistryURL + "/foo/.us&box",
		privateRegistryURL + "/bar/busybox", // match
		privateRegistryURL + "/notexistent/busybox",
		privateRegistryURL + "/foo/busybox", // match
	}, fmt.Sprintf("%s/bar/busybox		6-hell		%s\n", privateRegistryURL, helloWorldID)+
		fmt.Sprintf("%s/foo/busybox		2-busy		%s\n", privateRegistryURL, busyboxID)+
		fmt.Sprintf("%s/foo/busybox		3-busy		%s\n", privateRegistryURL, busyboxID)+
		fmt.Sprintf("%s/foo/busybox		5-hell		%s\n", privateRegistryURL, helloWorldID))
}

func (s *DockerRegistrySuite) TestTagListNotExistentRepository(c *check.C) {
	getImageIdOf := func(imageName string) string {
		cmd := exec.Command(dockerBinary, "images", "-q", "--no-trunc", imageName)
		out, _, err := runCommandWithOutput(cmd)
		if err != nil {
			c.Fatalf("failed to get image id for %q: %v", imageName, err)
		}
		ids := strings.Split(out, "\n")
		if len(ids) < 1 {
			c.Fatalf("image %q not found", imageName)
		}
		return ids[0]
	}
	busyboxID := getImageIdOf("busybox")

	dest := fmt.Sprintf("%s/foo/busybox", privateRegistryURL)
	cmd := exec.Command(dockerBinary, "tag", "busybox", dest)
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		c.Fatalf("failed to tag image %q as %q: error %v, output %q", "busybox", dest, err, out)
	}
	// list remote tags - shall list nothing
	assertTagListEqual(c, true, true, []string{dest}, "")

	// list local tags
	assertTagListEqual(c, false, true, []string{dest},
		fmt.Sprintf("%s/foo/busybox		latest		%s\n", privateRegistryURL, busyboxID))
}
