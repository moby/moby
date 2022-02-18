package session // import "github.com/moby/moby/integration/session"

import (
	"net/http"
	"testing"

	"github.com/moby/moby/api/types/versions"
	req "github.com/moby/moby/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestSessionCreate(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows", "FIXME")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.39"), "experimental in older versions")

	defer setupTest(t)()
	daemonHost := req.DaemonHost()

	res, body, err := req.Post("/session",
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
	skip.If(t, testEnv.OSType == "windows", "FIXME")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.39"), "experimental in older versions")

	defer setupTest(t)()
	daemonHost := req.DaemonHost()

	res, body, err := req.Post("/session", req.Host(daemonHost))
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(res.StatusCode, http.StatusBadRequest))
	buf, err := req.ReadBody(body)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(string(buf), "no upgrade"))

	res, body, err = req.Post("/session",
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
