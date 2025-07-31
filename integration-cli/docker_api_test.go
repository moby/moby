package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/moby/moby/v2/testutil"
	"github.com/moby/moby/v2/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type DockerAPISuite struct {
	ds *DockerSuite
}

func (s *DockerAPISuite) TearDownTest(ctx context.Context, t *testing.T) {
	s.ds.TearDownTest(ctx, t)
}

func (s *DockerAPISuite) OnTimeout(t *testing.T) {
	s.ds.OnTimeout(t)
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

func (s *DockerAPISuite) TestAPIErrorJSON(c *testing.T) {
	httpResp, body, err := request.Post(testutil.GetContext(c), "/containers/create", request.JSONBody(struct{}{}))
	assert.NilError(c, err)
	assert.Equal(c, httpResp.StatusCode, http.StatusBadRequest)
	assert.Assert(c, is.Contains(httpResp.Header.Get("Content-Type"), "application/json"))
	b, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Check(c, is.Contains(getErrorMessage(c, b), "config cannot be empty"))
}

func (s *DockerAPISuite) TestAPIErrorNotFoundJSON(c *testing.T) {
	// 404 is a different code path to normal errors, so test separately
	httpResp, body, err := request.Get(testutil.GetContext(c), "/notfound", request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, httpResp.StatusCode, http.StatusNotFound)
	assert.Assert(c, is.Contains(httpResp.Header.Get("Content-Type"), "application/json"))
	b, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Equal(c, getErrorMessage(c, b), "page not found")
}
