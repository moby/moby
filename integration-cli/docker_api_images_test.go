package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/request"
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
		status, b, err := request.SockRequest("GET", "/images/json?"+v.Encode(), nil, daemonHost())
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
	buildImageSuccessfully(c, "saveandload", withDockerfile("FROM busybox\nENV FOO bar"))
	id := getIDByName(c, "saveandload")

	res, body, err := request.SockRequestRaw("GET", "/images/"+id+"/get", nil, "", daemonHost())
	c.Assert(err, checker.IsNil)
	defer body.Close()
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	dockerCmd(c, "rmi", id)

	res, loadBody, err := request.SockRequestRaw("POST", "/images/load", body, "application/x-tar", daemonHost())
	c.Assert(err, checker.IsNil)
	defer loadBody.Close()
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	inspectOut := inspectField(c, id, "Id")
	c.Assert(strings.TrimSpace(string(inspectOut)), checker.Equals, id, check.Commentf("load did not work properly"))
}

func (s *DockerSuite) TestAPIImagesDelete(c *check.C) {
	if testEnv.DaemonPlatform() != "windows" {
		testRequires(c, Network)
	}
	name := "test-api-images-delete"
	buildImageSuccessfully(c, name, withDockerfile("FROM busybox\nENV FOO bar"))
	id := getIDByName(c, name)

	dockerCmd(c, "tag", name, "test:tag1")

	status, _, err := request.SockRequest("DELETE", "/images/"+id, nil, daemonHost())
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusConflict)

	status, _, err = request.SockRequest("DELETE", "/images/test:noexist", nil, daemonHost())
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotFound) //Status Codes:404 – no such image

	status, _, err = request.SockRequest("DELETE", "/images/test:tag1", nil, daemonHost())
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
}

func (s *DockerSuite) TestAPIImagesHistory(c *check.C) {
	if testEnv.DaemonPlatform() != "windows" {
		testRequires(c, Network)
	}
	name := "test-api-images-history"
	buildImageSuccessfully(c, name, withDockerfile("FROM busybox\nENV FOO bar"))
	id := getIDByName(c, name)

	status, body, err := request.SockRequest("GET", "/images/"+id+"/history", nil, daemonHost())
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var historydata []image.HistoryResponseItem
	err = json.Unmarshal(body, &historydata)
	c.Assert(err, checker.IsNil, check.Commentf("Error on unmarshal"))

	c.Assert(historydata, checker.Not(checker.HasLen), 0)
	c.Assert(historydata[0].Tags[0], checker.Equals, "test-api-images-history:latest")
}

// #14846
func (s *DockerSuite) TestAPIImagesSearchJSONContentType(c *check.C) {
	testRequires(c, Network)

	res, b, err := request.SockRequestRaw("GET", "/images/search?term=test", nil, "application/json", daemonHost())
	c.Assert(err, check.IsNil)
	b.Close()
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)
	c.Assert(res.Header.Get("Content-Type"), checker.Equals, "application/json")
}

// Test case for 30027: image size reported as -1 in v1.12 client against v1.13 daemon.
// This test checks to make sure both v1.12 and v1.13 client against v1.13 daemon get correct `Size` after the fix.
func (s *DockerSuite) TestAPIImagesSizeCompatibility(c *check.C) {
	status, b, err := request.SockRequest("GET", "/images/json", nil, daemonHost())
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
	var images []types.ImageSummary
	err = json.Unmarshal(b, &images)
	c.Assert(err, checker.IsNil)
	c.Assert(len(images), checker.Not(checker.Equals), 0)
	for _, image := range images {
		c.Assert(image.Size, checker.Not(checker.Equals), int64(-1))
	}

	type v124Image struct {
		ID          string `json:"Id"`
		ParentID    string `json:"ParentId"`
		RepoTags    []string
		RepoDigests []string
		Created     int64
		Size        int64
		VirtualSize int64
		Labels      map[string]string
	}
	status, b, err = request.SockRequest("GET", "/v1.24/images/json", nil, daemonHost())
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
	var v124Images []v124Image
	err = json.Unmarshal(b, &v124Images)
	c.Assert(err, checker.IsNil)
	c.Assert(len(v124Images), checker.Not(checker.Equals), 0)
	for _, image := range v124Images {
		c.Assert(image.Size, checker.Not(checker.Equals), int64(-1))
	}
}
