package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
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
