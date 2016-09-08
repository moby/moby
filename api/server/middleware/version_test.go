package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/server/httputils"
	"github.com/go-check/check"
	"golang.org/x/net/context"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestVersionMiddleware(c *check.C) {
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		c.Assert(httputils.VersionFromContext(ctx), check.Not(check.Equals), "")
		return nil
	}

	defaultVersion := "1.10.0"
	minVersion := "1.2.0"
	m := NewVersionMiddleware(defaultVersion, defaultVersion, minVersion)
	h := m.WrapHandler(handler)

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()
	c.Assert(h(ctx, resp, req, map[string]string{}), check.IsNil)
}

func (s *DockerSuite) TestVersionMiddlewareWithErrors(c *check.C) {
	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		c.Assert(httputils.VersionFromContext(ctx), check.Equals, "")
		return nil
	}

	defaultVersion := "1.10.0"
	minVersion := "1.2.0"
	m := NewVersionMiddleware(defaultVersion, defaultVersion, minVersion)
	h := m.WrapHandler(handler)

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()

	vars := map[string]string{"version": "0.1"}
	err := h(ctx, resp, req, vars)
	c.Assert(err, check.ErrorMatches, ".*client version 0.1 is too old. Minimum supported API version is 1.2.0.*")

	vars["version"] = "100000"
	err = h(ctx, resp, req, vars)
	c.Assert(err, check.ErrorMatches, ".*client is newer than server.*")
}
