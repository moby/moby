package image

import (
	"net/http"
	"testing"

	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// https://github.com/moby/moby/issues/14846
func TestImagesSearchJSONContentType(t *testing.T) {
	ctx := setupTest(t)

	res, body, err := request.Get(ctx, "/images/search?term=test", request.JSON)
	assert.NilError(t, err)
	body.Close()

	assert.Check(t, is.Equal(res.StatusCode, http.StatusOK))
	assert.Check(t, is.Equal(res.Header.Get("Content-type"), "application/json"))
}
