package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestAPIImagesFilter(c *check.C) {
	name := "utest:tag1"
	name2 := "utest/docker:tag2"
	name3 := "utest:5000/docker:tag3"
	for _, n := range []string{name, name2, name3} {
		dockerCmd(c, "tag", "busybox", n)
	}
	type image types.ImageSummary
	getImages := func(filter string) []image {
		v := url.Values{}
		v.Set("filter", filter)
		status, b, err := sockRequest("GET", "/images/json?"+v.Encode(), nil)
		c.Assert(err, checker.IsNil)
		c.Assert(status, checker.Equals, http.StatusOK)

		var images []image
		err = json.Unmarshal(b, &images)
		c.Assert(err, checker.IsNil)

		return images
	}

	//incorrect number of matches returned
	images := getImages("utest*/*")
	c.Assert(images[0].RepoTags, checker.HasLen, 2)

	images = getImages("utest")
	c.Assert(images[0].RepoTags, checker.HasLen, 1)

	images = getImages("utest*")
	c.Assert(images[0].RepoTags, checker.HasLen, 1)

	images = getImages("*5000*/*")
	c.Assert(images[0].RepoTags, checker.HasLen, 1)
}

func (s *DockerSuite) TestAPIImagesSaveAndLoad(c *check.C) {
	// TODO Windows to Windows CI: Investigate further why this test fails.
	testRequires(c, Network)
	testRequires(c, DaemonIsLinux)
	out, err := buildImage("saveandload", "FROM busybox\nENV FOO bar", false)
	c.Assert(err, checker.IsNil)
	id := strings.TrimSpace(out)

	res, body, err := sockRequestRaw("GET", "/images/"+id+"/get", nil, "")
	c.Assert(err, checker.IsNil)
	defer body.Close()
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	dockerCmd(c, "rmi", id)

	res, loadBody, err := sockRequestRaw("POST", "/images/load", body, "application/x-tar")
	c.Assert(err, checker.IsNil)
	defer loadBody.Close()
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	inspectOut := inspectField(c, id, "Id")
	c.Assert(strings.TrimSpace(string(inspectOut)), checker.Equals, id, check.Commentf("load did not work properly"))
}

func (s *DockerSuite) TestAPIImagesDelete(c *check.C) {
	if daemonPlatform != "windows" {
		testRequires(c, Network)
	}
	name := "test-api-images-delete"
	out, err := buildImage(name, "FROM busybox\nENV FOO bar", false)
	c.Assert(err, checker.IsNil)
	id := strings.TrimSpace(out)

	dockerCmd(c, "tag", name, "test:tag1")

	status, _, err := sockRequest("DELETE", "/images/"+id, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusConflict)

	status, _, err = sockRequest("DELETE", "/images/test:noexist", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotFound) //Status Codes:404 – no such image

	status, _, err = sockRequest("DELETE", "/images/test:tag1", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
}

func (s *DockerSuite) TestAPIImagesHistory(c *check.C) {
	if daemonPlatform != "windows" {
		testRequires(c, Network)
	}
	name := "test-api-images-history"
	out, err := buildImage(name, "FROM busybox\nENV FOO bar", false)
	c.Assert(err, checker.IsNil)

	id := strings.TrimSpace(out)

	status, body, err := sockRequest("GET", "/images/"+id+"/history", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var historydata []types.ImageHistory
	err = json.Unmarshal(body, &historydata)
	c.Assert(err, checker.IsNil, check.Commentf("Error on unmarshal"))

	c.Assert(historydata, checker.Not(checker.HasLen), 0)
	c.Assert(historydata[0].Tags[0], checker.Equals, "test-api-images-history:latest")
}

// #14846
func (s *DockerSuite) TestAPIImagesSearchJSONContentType(c *check.C) {
	testRequires(c, Network)

	res, b, err := sockRequestRaw("GET", "/images/search?term=test", nil, "application/json")
	c.Assert(err, check.IsNil)
	b.Close()
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)
	c.Assert(res.Header.Get("Content-Type"), checker.Equals, "application/json")
}
