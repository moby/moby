package container // import "github.com/docker/docker/integration/container"

import (
	"net/http"
	"runtime"
	"testing"

	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestContainerInvalidJSON tests that POST endpoints that expect a body return
// the correct error when sending invalid JSON requests.
func TestContainerInvalidJSON(t *testing.T) {
	t.Cleanup(setupTest(t))

	// POST endpoints that accept / expect a JSON body;
	endpoints := []string{
		"/commit",
		"/containers/create",
		"/containers/foobar/exec",
		"/containers/foobar/update",
		"/exec/foobar/start",
	}

	// windows doesnt support API < v1.24
	if runtime.GOOS != "windows" {
		endpoints = append(
			endpoints,
			"/v1.23/containers/foobar/copy",  // deprecated since 1.8 (API v1.20), errors out since 1.12 (API v1.24)
			"/v1.23/containers/foobar/start", // accepts a body on API < v1.24
		)
	}

	for _, ep := range endpoints {
		ep := ep
		t.Run(ep[1:], func(t *testing.T) {
			t.Parallel()

			t.Run("invalid content type", func(t *testing.T) {
				res, body, err := request.Post(ep, request.RawString("{}"), request.ContentType("text/plain"))
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "unsupported Content-Type header (text/plain): must be 'application/json'"))
			})

			t.Run("invalid JSON", func(t *testing.T) {
				res, body, err := request.Post(ep, request.RawString("{invalid json"), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "invalid JSON: invalid character 'i' looking for beginning of object key string"))
			})

			t.Run("extra content after JSON", func(t *testing.T) {
				res, body, err := request.Post(ep, request.RawString(`{} trailing content`), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "unexpected content after JSON"))
			})

			t.Run("empty body", func(t *testing.T) {
				// empty body should not produce an 500 internal server error, or
				// any 5XX error (this is assuming the request does not produce
				// an internal server error for another reason, but it shouldn't)
				res, _, err := request.Post(ep, request.RawString(``), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, res.StatusCode < http.StatusInternalServerError)
			})
		})
	}
}
