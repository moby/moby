package container // import "github.com/docker/docker/integration/container"

import (
	"net/http"
	"testing"

	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerInvalidJSON(t *testing.T) {
	defer setupTest(t)()

	endpoints := []string{
		"/containers/foobar/exec",
		"/exec/foobar/start",
	}

	for _, ep := range endpoints {
		ep := ep
		t.Run(ep, func(t *testing.T) {
			t.Parallel()

			res, body, err := request.Post(ep, request.RawString("{invalid json"), request.JSON)
			assert.NilError(t, err)
			assert.Equal(t, res.StatusCode, http.StatusBadRequest)

			buf, err := request.ReadBody(body)
			assert.NilError(t, err)
			assert.Check(t, is.Contains(string(buf), "invalid character 'i' looking for beginning of object key string"))

			res, body, err = request.Post(ep, request.JSON)
			assert.NilError(t, err)
			assert.Equal(t, res.StatusCode, http.StatusBadRequest)

			buf, err = request.ReadBody(body)
			assert.NilError(t, err)
			assert.Check(t, is.Contains(string(buf), "got EOF while reading request body"))
		})
	}
}
