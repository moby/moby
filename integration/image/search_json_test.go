package image

import (
	"net/http"
	"testing"

	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
)

func TestAPIImagesSearchJSONContentType(t *testing.T) {
	ctx := setupTest(t)

	res, b, err := request.Get(ctx, "/images/search?term=test", request.JSON)
	assert.NilError(t, err)
	b.Close()
	assert.Equal(t, res.StatusCode, http.StatusOK)
	assert.Equal(t, res.Header.Get("Content-Type"), "application/json")
}
