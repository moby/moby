package session // import "github.com/docker/docker/integration/session"

import (
	"net/http"
	"testing"

	req "github.com/docker/docker/integration-cli/request"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionCreate(t *testing.T) {
	skip.If(t, !testEnv.DaemonInfo.ExperimentalBuild)

	defer setupTest(t)()

	res, body, err := req.Post("/session", func(r *http.Request) error {
		r.Header.Set("X-Docker-Expose-Session-Uuid", "testsessioncreate") // so we don't block default name if something else is using it
		r.Header.Set("Upgrade", "h2c")
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, body.Close())
	assert.Equal(t, res.StatusCode, http.StatusSwitchingProtocols)
	assert.Equal(t, res.Header.Get("Upgrade"), "h2c")
}

func TestSessionCreateWithBadUpgrade(t *testing.T) {
	skip.If(t, !testEnv.DaemonInfo.ExperimentalBuild)

	res, body, err := req.Post("/session")
	require.NoError(t, err)
	assert.Equal(t, res.StatusCode, http.StatusBadRequest)
	buf, err := req.ReadBody(body)
	require.NoError(t, err)
	assert.Contains(t, string(buf), "no upgrade")

	res, body, err = req.Post("/session", func(r *http.Request) error {
		r.Header.Set("Upgrade", "foo")
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, res.StatusCode, http.StatusBadRequest)
	buf, err = req.ReadBody(body)
	require.NoError(t, err)
	assert.Contains(t, string(buf), "not supported")
}
