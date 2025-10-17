package system

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestAPIErrorNotFoundJSON(t *testing.T) {
	ctx := setupTest(t)

	// 404 is a different code path to normal errors, so test separately
	httpResp, body, err := request.Get(ctx, "/notfound", request.JSON)
	assert.NilError(t, err)
	defer body.Close()

	assert.Equal(t, httpResp.StatusCode, http.StatusNotFound)
	assert.Assert(t, is.Contains(httpResp.Header.Get("Content-Type"), "application/json"))

	b, err := request.ReadBody(body)
	assert.NilError(t, err)
	var errResp struct{ Message string }
	_ = json.Unmarshal(b, &errResp)
	assert.Equal(t, errResp.Message, "page not found")
}
