package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/middleware"

	"github.com/go-check/check"
	"golang.org/x/net/context"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestMiddlewares(c *check.C) {
	cfg := &Config{
		Version: "0.1omega2",
	}
	srv := &Server{
		cfg: cfg,
	}

	srv.UseMiddleware(middleware.NewVersionMiddleware("0.1omega2", api.DefaultVersion, api.MinVersion))

	req, _ := http.NewRequest("GET", "/containers/json", nil)
	resp := httptest.NewRecorder()
	ctx := context.Background()

	localHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		c.Assert(httputils.VersionFromContext(ctx), check.Not(check.Equals), "")
		c.Assert(w.Header().Get("Server"), check.Matches, "(?s).*Docker/0.1omega2.*")
		return nil
	}

	handlerFunc := srv.handlerWithGlobalMiddlewares(localHandler)
	c.Assert(handlerFunc(ctx, resp, req, map[string]string{}), check.IsNil)
}
