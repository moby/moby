package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
)

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
		res, _, err := request.Post(ctx, "/images/create?fromSrc="+te.fromSrc, request.JSON)
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

	imageList, err := apiclient.ImageList(testutil.GetContext(c), client.ImageListOptions{})
	assert.NilError(c, err)
	assert.Assert(c, len(imageList.Items) != 0)
	for _, img := range imageList.Items {
		assert.Assert(c, img.Size != int64(-1))
	}

	apiclient, err = client.New(client.FromEnv, client.WithAPIVersion("v1.24"))
	assert.NilError(c, err)
	defer apiclient.Close()

	v124Images, err := apiclient.ImageList(testutil.GetContext(c), client.ImageListOptions{})
	assert.NilError(c, err)
	assert.Assert(c, len(v124Images.Items) != 0)
	for _, img := range v124Images.Items {
		assert.Assert(c, img.Size != int64(-1))
	}
}
