package session // import "github.com/docker/docker/integration/session"

import (
	"net/http"
	"testing"

	req "github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestSessionCreate(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")

	ctx := setupTest(t)
	daemonHost := req.DaemonHost()

	res, body, err := req.Post(ctx, "/session",
		req.Host(daemonHost),
		req.With(func(r *http.Request) error {
			r.Header.Set("X-Docker-Expose-Session-Uuid", "testsessioncreate") // so we don't block default name if something else is using it
			r.Header.Set("Upgrade", "h2c")
			return nil
		}),
	)
	assert.NilError(t, err)
	assert.NilError(t, body.Close())
	assert.Check(t, is.DeepEqual(res.StatusCode, http.StatusSwitchingProtocols))
	assert.Check(t, is.Equal(res.Header.Get("Upgrade"), "h2c"))
}

func TestSessionCreateWithBadUpgrade(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")

	ctx := setupTest(t)
	daemonHost := req.DaemonHost()

	res, body, err := req.Post(ctx, "/session", req.Host(daemonHost))
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(res.StatusCode, http.StatusBadRequest))
	buf, err := req.ReadBody(body)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(string(buf), "no upgrade"))

	res, body, err = req.Post(ctx, "/session",
		req.Host(daemonHost),
		req.With(func(r *http.Request) error {
			r.Header.Set("Upgrade", "foo")
			return nil
		}),
	)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(res.StatusCode, http.StatusBadRequest))
	buf, err = req.ReadBody(body)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(string(buf), "not supported"))
}
