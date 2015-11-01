package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/go-check/check"
)

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

func dereferenceTagList(tagList []*types.RepositoryTag) []types.RepositoryTag {
	res := make([]types.RepositoryTag, len(tagList))
	for i, tag := range tagList {
		res[i] = *tag
	}
	return res
}

func assertTagListEqual(c *check.C, d *Daemon, remote, allowEmptyImageID bool, name, expName string, expTagList []types.RepositoryTag) {
	suffix := ""
	if remote {
		suffix = "?remote=1"
	}
	endpoint := fmt.Sprintf("/v1.20/images/%s/tags%s", name, suffix)
	status, body, err := func() (int, []byte, error) {
		if d == nil {
			return sockRequest("GET", endpoint, nil)
		}
		return d.sockRequest("GET", endpoint, nil)
	}()
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)

	var tagList types.RepositoryTagList
	if err = json.Unmarshal(body, &tagList); err != nil {
		c.Fatalf("failed to parse tag list: %v", err)
	}
	if allowEmptyImageID {
		for i, tag := range tagList.TagList {
			if tag.ImageID == "" && i < len(expTagList) {
				tag.ImageID = expTagList[i].ImageID
			}
		}
	}
	c.Assert(tagList.Name, check.Equals, expName)
	c.Assert(dereferenceTagList(tagList.TagList), check.DeepEquals, expTagList)
}

func (s *DockerRegistriesSuite) TestTagApiListRemoteRepository(c *check.C) {
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
	imgNames := []string{"busy", "hell"}
	for ri, reg := range []*testRegistryV2{s.reg1, s.reg2} {
		for i := 0; i < 5; i++ {
			tag := fmt.Sprintf("%s/foo/busybox:%d.%d-%s", reg.url, ri+1, i+1, imgNames[i/3])
			localTags = append(localTags, tag)
			if (ri+i)%3 == 0 {
				continue // upload 2/3 of registries
			}
			if out, err := d.Cmd("push", tag); err != nil {
				c.Fatalf("push of %q should have succeeded: %v, output: %s", tag, err, out)
			}
		}
	}

	assertTagListEqual(c, d, true, true, s.reg1.url+"/foo/busybox", s.reg1.url+"/foo/busybox",
		[]types.RepositoryTag{
			{"1.2-busy", busyboxID},
			{"1.3-busy", busyboxID},
			{"1.5-hell", helloWorldID},
		})

	assertTagListEqual(c, d, true, true, s.reg2.url+"/foo/busybox", s.reg2.url+"/foo/busybox",
		[]types.RepositoryTag{
			{"2.1-busy", busyboxID},
			{"2.2-busy", busyboxID},
			{"2.4-hell", helloWorldID},
			{"2.5-hell", helloWorldID},
		})

	assertTagListEqual(c, d, true, true, "foo/busybox", s.reg2.url+"/foo/busybox",
		[]types.RepositoryTag{
			{"2.1-busy", busyboxID},
			{"2.2-busy", busyboxID},
			{"2.4-hell", helloWorldID},
			{"2.5-hell", helloWorldID},
		})

	assertTagListEqual(c, d, false, false, s.reg1.url+"/foo/busybox", s.reg1.url+"/foo/busybox",
		[]types.RepositoryTag{
			{"1.1-busy", busyboxID},
			{"1.2-busy", busyboxID},
			{"1.3-busy", busyboxID},
			{"1.4-hell", helloWorldID},
			{"1.5-hell", helloWorldID},
		})

	assertTagListEqual(c, d, false, false, s.reg2.url+"/foo/busybox", s.reg2.url+"/foo/busybox",
		[]types.RepositoryTag{
			{"2.1-busy", busyboxID},
			{"2.2-busy", busyboxID},
			{"2.3-busy", busyboxID},
			{"2.4-hell", helloWorldID},
			{"2.5-hell", helloWorldID},
		})

	// now delete all local images
	if out, err := d.Cmd("rmi", localTags...); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", localTags, err, out)
	}

	// and try again
	assertTagListEqual(c, d, true, true, "foo/busybox", s.reg2.url+"/foo/busybox",
		[]types.RepositoryTag{
			{"2.1-busy", busyboxID},
			{"2.2-busy", busyboxID},
			{"2.4-hell", helloWorldID},
			{"2.5-hell", helloWorldID},
		})

	assertTagListEqual(c, d, false, true, s.reg1.url+"/foo/busybox", s.reg1.url+"/foo/busybox",
		[]types.RepositoryTag{
			{"1.2-busy", busyboxID},
			{"1.3-busy", busyboxID},
			{"1.5-hell", helloWorldID},
		})
}

func (s *DockerRegistrySuite) TestTagApiListNotExistentRepository(c *check.C) {
	d := NewDaemon(c)
	if err := d.StartWithBusybox(); err != nil {
		c.Fatalf("we should have been able to start the daemon: %v", err)
	}
	defer d.Stop()

	busyboxID := d.getAndTestImageEntry(c, 1, "busybox", "").id

	repoName := fmt.Sprintf("%s/foo/busybox", s.reg.url)
	if out, err := d.Cmd("tag", "busybox", repoName); err != nil {
		c.Fatalf("failed to tag image %q as %q: error %v, output %q", "busybox", repoName, err, out)
	}
	// list remote tags - shall list nothing
	assertTagListEqual(c, d, true, true, repoName, repoName, []types.RepositoryTag{})

	// list local tags
	assertTagListEqual(c, d, false, false, repoName, repoName,
		[]types.RepositoryTag{
			{"latest", busyboxID},
		})
}
