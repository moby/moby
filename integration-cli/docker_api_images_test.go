package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/docker/docker/internal/test/request"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/go-check/check"
	"fmt"
)

func (s *DockerSuite) TestAPIImagesFilter(c *check.C) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
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

func (s *DockerRegistrySuite) TestAPIImagesFilterHasDigest(c *check.C) {
	repoName := fmt.Sprintf("%v/busybox:mytag", privateRegistryURL)
	repoName2 := fmt.Sprintf("%v/busybox:mytag2", privateRegistryURL)
	repoName3 := fmt.Sprintf("%v/utest:tag1", privateRegistryURL)
	for _, n := range []string{repoName, repoName2, repoName3} {
		dockerCmd(c, "tag", "busybox", n)
	}

	// Push only one image to compute the digest
	dockerCmd(c, "push", repoName)
	dockerCmd(c, "push", repoName2)

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

	images := getImages(privateRegistryURL + "/utest")
	c.Assert(images[0].RepoDigests, checker.HasLen, 0)

	images = getImages(privateRegistryURL + "/busybox")
	c.Assert(images[0].RepoDigests, checker.HasLen, 1)
	c.Assert(images[0].RepoTags, checker.HasLen, 2)
}

func (s *DockerSuite) TestAPIImagesSaveAndLoad(c *check.C) {
	if runtime.GOOS == "windows" {
		v, err := kernel.GetKernelVersion()
		c.Assert(err, checker.IsNil)
		build, _ := strconv.Atoi(strings.Split(strings.SplitN(v.String(), " ", 3)[2][1:], ".")[0])
		if build == 16299 {
			c.Skip("Temporarily disabled on RS3 builds")
		}
	}

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
	cli, err := client.NewClientWithOpts(client.FromEnv)
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
	cli, err := client.NewClientWithOpts(client.FromEnv)
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
	if runtime.GOOS == "windows" {
		v, err := kernel.GetKernelVersion()
		c.Assert(err, checker.IsNil)
		build, _ := strconv.Atoi(strings.Split(strings.SplitN(v.String(), " ", 3)[2][1:], ".")[0])
		if build == 16299 {
			c.Skip("Temporarily disabled on RS3 builds")
		}
	}

	testRequires(c, Network, testEnv.IsLocalDaemon)

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
