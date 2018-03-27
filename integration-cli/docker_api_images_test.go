package main

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/docker/docker/integration-cli/request"
	"github.com/go-check/check"
	"golang.org/x/net/context"
)

func (s *DockerSuite) TestAPIImagesFilter(c *check.C) {
	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	name := "utest:tag1"
	name2 := "utest/docker:tag2"
	name3 := "utest:5000/docker:tag3"
	for _, n := range []string{name, name2, name3} {
		dockerCmd(c, "tag", "busybox", n)
	}
	getImages := func(filter string) []types.ImageSummary {
		filters := filters.NewArgs()
		filters.Add("reference", filter)
		options := types.ImageListOptions{
			All:     false,
			Filters: filters,
		}
		images, err := cli.ImageList(context.Background(), options)
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
	testRequires(c, Network)
	buildImageSuccessfully(c, "saveandload", build.WithDockerfile("FROM busybox\nENV FOO bar"))
	id := getIDByName(c, "saveandload")

	res, body, err := request.Get("/images/" + id + "/get")
	c.Assert(err, checker.IsNil)
	defer body.Close()
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	dockerCmd(c, "rmi", id)

	res, loadBody, err := request.Post("/images/load", request.RawContent(body), request.ContentType("application/x-tar"))
	c.Assert(err, checker.IsNil)
	defer loadBody.Close()
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	inspectOut := cli.InspectCmd(c, id, cli.Format(".Id")).Combined()
	c.Assert(strings.TrimSpace(string(inspectOut)), checker.Equals, id, check.Commentf("load did not work properly"))
}

func (s *DockerSuite) TestAPIImagesDelete(c *check.C) {
	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	if testEnv.OSType != "windows" {
		testRequires(c, Network)
	}
	name := "test-api-images-delete"
	buildImageSuccessfully(c, name, build.WithDockerfile("FROM busybox\nENV FOO bar"))
	id := getIDByName(c, name)

	dockerCmd(c, "tag", name, "test:tag1")

	_, err = cli.ImageRemove(context.Background(), id, types.ImageRemoveOptions{})
	c.Assert(err.Error(), checker.Contains, "unable to delete")

	_, err = cli.ImageRemove(context.Background(), "test:noexist", types.ImageRemoveOptions{})
	c.Assert(err.Error(), checker.Contains, "No such image")

	_, err = cli.ImageRemove(context.Background(), "test:tag1", types.ImageRemoveOptions{})
	c.Assert(err, checker.IsNil)
}

func (s *DockerSuite) TestAPIImagesHistory(c *check.C) {
	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	if testEnv.OSType != "windows" {
		testRequires(c, Network)
	}
	name := "test-api-images-history"
	buildImageSuccessfully(c, name, build.WithDockerfile("FROM busybox\nENV FOO bar"))
	id := getIDByName(c, name)

	historydata, err := cli.ImageHistory(context.Background(), id)
	c.Assert(err, checker.IsNil)

	c.Assert(historydata, checker.Not(checker.HasLen), 0)
	var found bool
	for _, tag := range historydata[0].Tags {
		if tag == "test-api-images-history:latest" {
			found = true
			break
		}
	}
	c.Assert(found, checker.True)
}

func (s *DockerSuite) TestAPIImagesImportBadSrc(c *check.C) {
	testRequires(c, Network, SameHostDaemon)

	server := httptest.NewServer(http.NewServeMux())
	defer server.Close()

	tt := []struct {
		statusExp int
		fromSrc   string
	}{
		{http.StatusNotFound, server.URL + "/nofile.tar"},
		{http.StatusNotFound, strings.TrimPrefix(server.URL, "http://") + "/nofile.tar"},
		{http.StatusNotFound, strings.TrimPrefix(server.URL, "http://") + "%2Fdata%2Ffile.tar"},
		{http.StatusInternalServerError, "%2Fdata%2Ffile.tar"},
	}

	for _, te := range tt {
		res, _, err := request.Post(strings.Join([]string{"/images/create?fromSrc=", te.fromSrc}, ""), request.JSON)
		c.Assert(err, check.IsNil)
		c.Assert(res.StatusCode, checker.Equals, te.statusExp)
		c.Assert(res.Header.Get("Content-Type"), checker.Equals, "application/json")
	}

}

// #14846
func (s *DockerSuite) TestAPIImagesSearchJSONContentType(c *check.C) {
	testRequires(c, Network)

	res, b, err := request.Get("/images/search?term=test", request.JSON)
	c.Assert(err, check.IsNil)
	b.Close()
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)
	c.Assert(res.Header.Get("Content-Type"), checker.Equals, "application/json")
}

// Test case for 30027: image size reported as -1 in v1.12 client against v1.13 daemon.
// This test checks to make sure both v1.12 and v1.13 client against v1.13 daemon get correct `Size` after the fix.
func (s *DockerSuite) TestAPIImagesSizeCompatibility(c *check.C) {
	apiclient := testEnv.APIClient()
	defer apiclient.Close()

	images, err := apiclient.ImageList(context.Background(), types.ImageListOptions{})
	c.Assert(err, checker.IsNil)
	c.Assert(len(images), checker.Not(checker.Equals), 0)
	for _, image := range images {
		c.Assert(image.Size, checker.Not(checker.Equals), int64(-1))
	}

	apiclient, err = client.NewClientWithOpts(client.FromEnv, client.WithVersion("v1.24"))
	c.Assert(err, checker.IsNil)
	defer apiclient.Close()

	v124Images, err := apiclient.ImageList(context.Background(), types.ImageListOptions{})
	c.Assert(err, checker.IsNil)
	c.Assert(len(v124Images), checker.Not(checker.Equals), 0)
	for _, image := range v124Images {
		c.Assert(image.Size, checker.Not(checker.Equals), int64(-1))
	}
}
