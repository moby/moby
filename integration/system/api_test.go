package system

import (
	"net/http"
	"testing"

	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestAPIErrorNotFoundJSON(t *testing.T) {
	ctx := setupTest(t)

	// 404 is a different code path to normal errors, so test separately
	httpResp, body, err := request.Get(ctx, "/notfound", request.JSON)
	assert.NilError(t, err)
	assert.Equal(t, httpResp.StatusCode, http.StatusNotFound)
	assert.Assert(t, is.Contains(httpResp.Header.Get("Content-Type"), "application/json"))

	b, err := request.ReadBody(body)
	assert.NilError(t, err)
	assert.Equal(t, getErrorMessage(t, b), "page not found")
}
