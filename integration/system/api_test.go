package system

import (
	"net/http"
	"testing"

	"github.com/moby/moby/api/types/common"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
)

func TestAPIErrorNotFoundJSON(t *testing.T) {
	ctx := setupTest(t)

	// 404 is a different code path to normal errors, so test separately
	httpResp, _, err := request.Get(ctx, "/notfound", request.JSON)
	assert.NilError(t, err)
	assert.Equal(t, httpResp.StatusCode, http.StatusNotFound)

	var respErr common.ErrorResponse
	assert.NilError(t, request.ReadJSONResponse(httpResp, &respErr))
	assert.Error(t, respErr, "page not found")
}
