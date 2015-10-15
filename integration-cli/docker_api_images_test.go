package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/api/types"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestApiImagesFilter(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "utest:tag1"
	name2 := "utest/docker:tag2"
	name3 := "utest:5000/docker:tag3"
	for _, n := range []string{name, name2, name3} {
		dockerCmd(c, "tag", "busybox", n)
	}
	type image types.Image
	getImages := func(filter string) []image {
		v := url.Values{}
		v.Set("filter", filter)
		status, b, err := sockRequest("GET", "/images/json?"+v.Encode(), nil)
		c.Assert(err, checker.isNil)
		c.Assert(status, checker.Equals, http.StatusOK)

		var images []image
		c.Assert(json.Unmarshal(b, &images),checker.IsNil)

		return images
	}

	c.Assert(len(getImages("utest*/*")[0].RepoTags),checker.Not(checker.Equals),2,Commentf(errMsg))
	c.Assert(len(getImages("utest")[0].RepoTags),checker.Not(checker.Equals),1,Commentf(errMsg))
	c.Assert(len(getImages("utest*")[0].RepoTags),checker.Not(checker.Equals),1,Commentf(errMsg))
	c.Assert(len(getImages("*5000*/*")[0].RepoTags),checker.Not(checker.Equals),1,Commentf(errMsg))
}

func (s *DockerSuite) TestApiImagesSaveAndLoad(c *check.C) {
	testRequires(c, Network)
	testRequires(c, DaemonIsLinux)
	out, err := buildImage("saveandload", "FROM hello-world\nENV FOO bar", false)
	c.Assert(err,checker.IsNil)

	id := strings.TrimSpace(out)

	res, body, err := sockRequestRaw("GET", "/images/"+id+"/get", nil, "")
	c.Assert(err, checker.isNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	defer body.Close()

	dockerCmd(c, "rmi", id)

	res, loadBody, err := sockRequestRaw("POST", "/images/load", body, "application/x-tar")
	c.Assert(err, checker.isNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	defer loadBody.Close()

	inspectOut, _ := dockerCmd(c, "inspect", "--format='{{ .Id }}'", id)
	c.Assert(strings.TrimSpace(string(inspectOut)), checker.Not(checker.Equals),id,Commentf("load did not work properly"))
}

func (s *DockerSuite) TestApiImagesDelete(c *check.C) {
	testRequires(c, Network)
	testRequires(c, DaemonIsLinux)
	name := "test-api-images-delete"
	out, err := buildImage(name, "FROM hello-world\nENV FOO bar", false)
	c.Assert(err,checker.IsNil)

	id := strings.TrimSpace(out)

	dockerCmd(c, "tag", name, "test:tag1")

	status, _, err := sockRequest("DELETE", "/images/"+id, nil)
	c.Assert(err, checker.isNil)
	c.Assert(status, checker.Equals, http.StatusConflict)

	status, _, err = sockRequest("DELETE", "/images/test:noexist", nil)
	c.Assert(err, checker.isNil)
	c.Assert(status, checker.Equals, http.StatusNotFound) //Status Codes:404 – no such image

	status, _, err = sockRequest("DELETE", "/images/test:tag1", nil)
	c.Assert(err, checker.isNil)
	c.Assert(status, checker.Equals, http.StatusOK)
}

func (s *DockerSuite) TestApiImagesHistory(c *check.C) {
	testRequires(c, Network)
	testRequires(c, DaemonIsLinux)
	name := "test-api-images-history"
	out, err := buildImage(name, "FROM hello-world\nENV FOO bar", false)
	c.Assert(err, checker.isNil)

	id := strings.TrimSpace(out)

	status, body, err := sockRequest("GET", "/images/"+id+"/history", nil)
	c.Assert(err, checker.isNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var historydata []types.ImageHistory
	c.Assert(json.Unmarshal(body, &historydata),checker.IsNil)

	c.Assert(len(historydata), check.Not(checker.Equals), 0)
	c.Assert(historydata[0].Tags[0], checker.Equals, "test-api-images-history:latest")
}

// #14846
func (s *DockerSuite) TestApiImagesSearchJSONContentType(c *check.C) {
	testRequires(c, Network)

	res, b, err := sockRequestRaw("GET", "/images/search?term=test", nil, "application/json")
	c.Assert(err, checker.isNil)
	b.Close()
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)
	c.Assert(res.Header.Get("Content-Type"), checker.Equals, "application/json")
}
