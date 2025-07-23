package image

import (
	"net/http"
	"testing"

	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestSearchContentType verifies that the search endpoint
// returns the correct content-type.
//
// Regression test for https://github.com/moby/moby/issues/14846
func TestSearchContentType(t *testing.T) {
	ctx := setupTest(t)

	res, b, err := request.Get(ctx, "/images/search?term=test", request.JSON)
	assert.NilError(t, err)
	_ = b.Close()

	assert.Check(t, is.Equal(res.StatusCode, http.StatusOK))
	assert.Check(t, is.Equal(res.Header.Get("Content-Type"), "application/json"))
}
