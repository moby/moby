package system // import "github.com/docker/docker/integration/system"

import (
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
)

func TestPingCacheHeaders(t *testing.T) {
	defer setupTest(t)()

	res, _, err := request.Get("/_ping")
	assert.NilError(t, err)
	assert.Equal(t, res.StatusCode, http.StatusOK)

	assert.Equal(t, hdr(res, "Cache-Control"), "no-cache, no-store, must-revalidate")
	assert.Equal(t, hdr(res, "Pragma"), "no-cache")
}

func hdr(res *http.Response, name string) string {
	val, ok := res.Header[http.CanonicalHeaderKey(name)]
	if !ok || len(val) == 0 {
		return ""
	}
	return strings.Join(val, ", ")
}
