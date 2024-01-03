package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
)

func (s *DockerAPISuite) TestAPIImagesSaveAndLoad(c *testing.T) {
	testRequires(c, Network)
	buildImageSuccessfully(c, "saveandload", build.WithDockerfile("FROM busybox\nENV FOO bar"))
	id := getIDByName(c, "saveandload")

	ctx := testutil.GetContext(c)
	res, body, err := request.Get(ctx, "/images/"+id+"/get")
	assert.NilError(c, err)
	defer body.Close()
	assert.Equal(c, res.StatusCode, http.StatusOK)

	cli.DockerCmd(c, "rmi", id)

	res, loadBody, err := request.Post(ctx, "/images/load", request.RawContent(body), request.ContentType("application/x-tar"))
	assert.NilError(c, err)
	defer loadBody.Close()
	assert.Equal(c, res.StatusCode, http.StatusOK)

	inspectOut := cli.InspectCmd(c, id, cli.Format(".Id")).Combined()
	assert.Equal(c, strings.TrimSpace(inspectOut), id, "load did not work properly")
}

func (s *DockerAPISuite) TestAPIImagesDelete(c *testing.T) {
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	if testEnv.DaemonInfo.OSType != "windows" {
		testRequires(c, Network)
	}
	name := "test-api-images-delete"
	buildImageSuccessfully(c, name, build.WithDockerfile("FROM busybox\nENV FOO bar"))
	id := getIDByName(c, name)

	cli.DockerCmd(c, "tag", name, "test:tag1")

	_, err = apiClient.ImageRemove(testutil.GetContext(c), id, types.ImageRemoveOptions{})
	assert.ErrorContains(c, err, "unable to delete")

	_, err = apiClient.ImageRemove(testutil.GetContext(c), "test:noexist", types.ImageRemoveOptions{})
	assert.ErrorContains(c, err, "No such image")

	_, err = apiClient.ImageRemove(testutil.GetContext(c), "test:tag1", types.ImageRemoveOptions{})
	assert.NilError(c, err)
}

func (s *DockerAPISuite) TestAPIImagesHistory(c *testing.T) {
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	if testEnv.DaemonInfo.OSType != "windows" {
		testRequires(c, Network)
	}
	name := "test-api-images-history"
	buildImageSuccessfully(c, name, build.WithDockerfile("FROM busybox\nENV FOO bar"))
	id := getIDByName(c, name)

	historydata, err := apiClient.ImageHistory(testutil.GetContext(c), id)
	assert.NilError(c, err)

	assert.Assert(c, len(historydata) != 0)
	var found bool
	for _, tag := range historydata[0].Tags {
		if tag == "test-api-images-history:latest" {
			found = true
			break
		}
	}
	assert.Assert(c, found)
}

func (s *DockerAPISuite) TestAPIImagesImportBadSrc(c *testing.T) {
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

	ctx := testutil.GetContext(c)
	for _, te := range tt {
		res, _, err := request.Post(ctx, strings.Join([]string{"/images/create?fromSrc=", te.fromSrc}, ""), request.JSON)
		assert.NilError(c, err)
		assert.Equal(c, res.StatusCode, te.statusExp)
		assert.Equal(c, res.Header.Get("Content-Type"), "application/json")
	}
}

// #14846
func (s *DockerAPISuite) TestAPIImagesSearchJSONContentType(c *testing.T) {
	testRequires(c, Network)

	res, b, err := request.Get(testutil.GetContext(c), "/images/search?term=test", request.JSON)
	assert.NilError(c, err)
	b.Close()
	assert.Equal(c, res.StatusCode, http.StatusOK)
	assert.Equal(c, res.Header.Get("Content-Type"), "application/json")
}

// Test case for 30027: image size reported as -1 in v1.12 client against v1.13 daemon.
// This test checks to make sure both v1.12 and v1.13 client against v1.13 daemon get correct `Size` after the fix.
func (s *DockerAPISuite) TestAPIImagesSizeCompatibility(c *testing.T) {
	apiclient := testEnv.APIClient()
	defer apiclient.Close()

	images, err := apiclient.ImageList(testutil.GetContext(c), types.ImageListOptions{})
	assert.NilError(c, err)
	assert.Assert(c, len(images) != 0)
	for _, image := range images {
		assert.Assert(c, image.Size != int64(-1))
	}

	apiclient, err = client.NewClientWithOpts(client.FromEnv, client.WithVersion("v1.24"))
	assert.NilError(c, err)
	defer apiclient.Close()

	v124Images, err := apiclient.ImageList(testutil.GetContext(c), types.ImageListOptions{})
	assert.NilError(c, err)
	assert.Assert(c, len(v124Images) != 0)
	for _, image := range v124Images {
		assert.Assert(c, image.Size != int64(-1))
	}
}
