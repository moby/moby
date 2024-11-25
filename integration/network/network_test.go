package network // import "github.com/docker/docker/integration/network"

import (
	"encoding/json"
	"net/http"
	"testing"

	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestNetworkInvalidJSON tests that POST endpoints that expect a body return
// the correct error when sending invalid JSON requests.
func TestNetworkInvalidJSON(t *testing.T) {
	ctx := setupTest(t)

	// POST endpoints that accept / expect a JSON body;
	endpoints := []string{
		"/networks/create",
		"/networks/bridge/connect",
		"/networks/bridge/disconnect",
	}

	for _, ep := range endpoints {
		t.Run(ep[1:], func(t *testing.T) {
			t.Parallel()
			ctx := testutil.StartSpan(ctx, t)

			t.Run("invalid content type", func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				res, body, err := request.Post(ctx, ep, request.RawString("{}"), request.ContentType("text/plain"))
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "unsupported Content-Type header (text/plain): must be 'application/json'"))
			})

			t.Run("invalid JSON", func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				res, body, err := request.Post(ctx, ep, request.RawString("{invalid json"), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "invalid JSON: invalid character 'i' looking for beginning of object key string"))
			})

			t.Run("extra content after JSON", func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				res, body, err := request.Post(ctx, ep, request.RawString(`{} trailing content`), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "unexpected content after JSON"))
			})

			t.Run("empty body", func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				// empty body should not produce an 500 internal server error, or
				// any 5XX error (this is assuming the request does not produce
				// an internal server error for another reason, but it shouldn't)
				res, _, err := request.Post(ctx, ep, request.RawString(``), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, res.StatusCode < http.StatusInternalServerError)
			})
		})
	}
}

// TestNetworkList verifies that /networks returns a list of networks either
// with, or without a trailing slash (/networks/). Regression test for https://github.com/moby/moby/issues/24595
func TestNetworkList(t *testing.T) {
	ctx := setupTest(t)

	endpoints := []string{
		"/networks",
		"/networks/",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			t.Parallel()

			res, body, err := request.Get(ctx, ep, request.JSON)
			assert.NilError(t, err)
			assert.Equal(t, res.StatusCode, http.StatusOK)

			buf, err := request.ReadBody(body)
			assert.NilError(t, err)
			var nws []networktypes.Inspect
			err = json.Unmarshal(buf, &nws)
			assert.NilError(t, err)
			assert.Assert(t, len(nws) > 0)
		})
	}
}
