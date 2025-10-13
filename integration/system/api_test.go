package system

import (
	"net/http"
	"testing"

	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestAPIErrorJSON(t *testing.T) {
	ctx := setupTest(t)
	httpResp, body, err := request.Post(ctx, "/containers/create", request.JSONBody(struct{}{}))
	assert.NilError(t, err)
	assert.Equal(t, httpResp.StatusCode, http.StatusBadRequest)
	assert.Assert(t, is.Contains(httpResp.Header.Get("Content-Type"), "application/json"))
	b, err := request.ReadBody(body)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(getErrorMessage(t, b), "config cannot be empty"))
}
