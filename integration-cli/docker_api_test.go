package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
)

type DockerAPISuite struct {
	ds *DockerSuite
}

func (s *DockerAPISuite) TearDownTest(ctx context.Context, c *testing.T) {
	s.ds.TearDownTest(ctx, c)
}

func (s *DockerAPISuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerAPISuite) TestAPIOptionsRoute(c *testing.T) {
	resp, _, err := request.Do(testutil.GetContext(c), "/", request.Method(http.MethodOptions))
	assert.NilError(c, err)
	assert.Equal(c, resp.StatusCode, http.StatusOK)
}

func (s *DockerAPISuite) TestAPIGetEnabledCORS(c *testing.T) {
	res, body, err := request.Get(testutil.GetContext(c), "/version")
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusOK)
	body.Close()
	// TODO: @runcom incomplete tests, why old integration tests had this headers
	// and here none of the headers below are in the response?
	// c.Log(res.Header)
	// assert.Equal(c, res.Header.Get("Access-Control-Allow-Origin"), "*")
	// assert.Equal(c, res.Header.Get("Access-Control-Allow-Headers"), "Origin, X-Requested-With, Content-Type, Accept, X-Registry-Auth")
}

func (s *DockerAPISuite) TestAPIClientVersionOldNotSupported(c *testing.T) {
	if testEnv.DaemonInfo.OSType != runtime.GOOS {
		c.Skip("Daemon platform doesn't match test platform")
	}

	major, minor, _ := strings.Cut(testEnv.DaemonVersion.MinAPIVersion, ".")
	vMinInt, err := strconv.Atoi(minor)
	assert.NilError(c, err)
	vMinInt--
	version := fmt.Sprintf("%s.%d", major, vMinInt)

	resp, body, err := request.Get(testutil.GetContext(c), "/v"+version+"/version")
	assert.NilError(c, err)
	defer body.Close()
	assert.Equal(c, resp.StatusCode, http.StatusBadRequest)
	expected := fmt.Sprintf("client version %s is too old. Minimum supported API version is %s, please upgrade your client to a newer version", version, testEnv.DaemonVersion.MinAPIVersion)
	content, err := io.ReadAll(body)
	assert.NilError(c, err)
	assert.Equal(c, strings.TrimSpace(string(content)), expected)
}

func (s *DockerAPISuite) TestAPIErrorJSON(c *testing.T) {
	httpResp, body, err := request.Post(testutil.GetContext(c), "/containers/create", request.JSONBody(struct{}{}))
	assert.NilError(c, err)
	assert.Equal(c, httpResp.StatusCode, http.StatusBadRequest)
	assert.Assert(c, strings.Contains(httpResp.Header.Get("Content-Type"), "application/json"))
	b, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Equal(c, getErrorMessage(c, b), runconfig.ErrEmptyConfig.Error())
}

func (s *DockerAPISuite) TestAPIErrorPlainText(c *testing.T) {
	// Windows requires API 1.25 or later. This test is validating a behaviour which was present
	// in v1.23, but changed in 1.24, hence not applicable on Windows. See apiVersionSupportsJSONErrors
	testRequires(c, DaemonIsLinux)
	httpResp, body, err := request.Post(testutil.GetContext(c), "/v1.23/containers/create", request.JSONBody(struct{}{}))
	assert.NilError(c, err)
	assert.Equal(c, httpResp.StatusCode, http.StatusBadRequest)
	assert.Assert(c, strings.Contains(httpResp.Header.Get("Content-Type"), "text/plain"))
	b, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Equal(c, strings.TrimSpace(string(b)), runconfig.ErrEmptyConfig.Error())
}

func (s *DockerAPISuite) TestAPIErrorNotFoundJSON(c *testing.T) {
	// 404 is a different code path to normal errors, so test separately
	httpResp, body, err := request.Get(testutil.GetContext(c), "/notfound", request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, httpResp.StatusCode, http.StatusNotFound)
	assert.Assert(c, strings.Contains(httpResp.Header.Get("Content-Type"), "application/json"))
	b, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Equal(c, getErrorMessage(c, b), "page not found")
}

func (s *DockerAPISuite) TestAPIErrorNotFoundPlainText(c *testing.T) {
	httpResp, body, err := request.Get(testutil.GetContext(c), "/v1.23/notfound", request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, httpResp.StatusCode, http.StatusNotFound)
	assert.Assert(c, strings.Contains(httpResp.Header.Get("Content-Type"), "text/plain"))
	b, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Equal(c, strings.TrimSpace(string(b)), "page not found")
}
