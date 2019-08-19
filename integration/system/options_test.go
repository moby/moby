package system

import (
	"net/http"
	"testing"

	req "github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
)

func TestAPIOptionsRoute(t *testing.T) {
	defer setupTest(t)()

	resp, _, err := req.Do("/", req.Method(http.MethodOptions))
	assert.NilError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestAPIGetEnabledCORS(t *testing.T) {
	resp, body, err := req.Get("/version")
	assert.NilError(t, err)
	defer body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}
