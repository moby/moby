package system

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	"github.com/docker/docker/internal/test/daemon"

	req "github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
)

func TestAPIOptionsRoute(t *testing.T) {
	defer setupTest(t)()

	resp, _, err := req.Do("/", req.Method(http.MethodOptions))
	assert.NilError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestAPIGetEnabledCORS(t *testing.T) {
	defer setupTest(t)()

	d := daemon.New(t)

	d.Start(t, "--api-cors-header='*'")
	defer d.Stop(t)

	daemonURL, err := url.Parse(d.Sock())
	assert.NilError(t, err)

	conn, err := net.DialTimeout(daemonURL.Scheme, daemonURL.Path, time.Second*10)
	assert.NilError(t, err)

	c := httputil.NewClientConn(conn, nil)

	req, err := http.NewRequest("GET", "/_ping", nil)
	assert.NilError(t, err)

	resp, err := c.Do(req)
	assert.NilError(t, err)

	assert.Equal(t, resp.StatusCode, http.StatusOK)

	assert.Equal(t, len(resp.Header["Access-Control-Allow-Origin"]), 1)
	assert.Equal(t, resp.Header["Access-Control-Allow-Origin"][0], "'*'")
}
