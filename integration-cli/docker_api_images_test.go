package main

import (
	"net/http"
	"testing"

	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
)

// #14846
func (s *DockerAPISuite) TestAPIImagesSearchJSONContentType(c *testing.T) {
	testRequires(c, Network)

	res, b, err := request.Get(testutil.GetContext(c), "/images/search?term=test", request.JSON)
	assert.NilError(c, err)
	b.Close()
	assert.Equal(c, res.StatusCode, http.StatusOK)
	assert.Equal(c, res.Header.Get("Content-Type"), "application/json")
}
