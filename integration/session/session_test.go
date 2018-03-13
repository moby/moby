package session // import "github.com/docker/docker/integration/session"

import (
	"net/http"
	"testing"

	req "github.com/docker/docker/integration-cli/request"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/gotestyourself/gotestyourself/skip"
)

func TestSessionCreate(t *testing.T) {
	skip.If(t, !testEnv.DaemonInfo.ExperimentalBuild)

	defer setupTest(t)()

	res, body, err := req.Post("/session", func(r *http.Request) error {
		r.Header.Set("X-Docker-Expose-Session-Uuid", "testsessioncreate") // so we don't block default name if something else is using it
		r.Header.Set("Upgrade", "h2c")
		return nil
	})
	assert.NilError(t, err)
	assert.NilError(t, body.Close())
	assert.Check(t, is.DeepEqual(res.StatusCode, http.StatusSwitchingProtocols))
	assert.Check(t, is.Equal(res.Header.Get("Upgrade"), "h2c"))
}

func TestSessionCreateWithBadUpgrade(t *testing.T) {
	skip.If(t, !testEnv.DaemonInfo.ExperimentalBuild)

	res, body, err := req.Post("/session")
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(res.StatusCode, http.StatusBadRequest))
	buf, err := req.ReadBody(body)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(string(buf), "no upgrade"))

	res, body, err = req.Post("/session", func(r *http.Request) error {
		r.Header.Set("Upgrade", "foo")
		return nil
	})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(res.StatusCode, http.StatusBadRequest))
	buf, err = req.ReadBody(body)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(string(buf), "not supported"))
}
