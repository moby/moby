package system // import "github.com/docker/docker/integration/system"

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	req "github.com/docker/docker/integration-cli/request"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptionsRoute(t *testing.T) {
	resp, _, err := req.Do("/", req.Method(http.MethodOptions))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestGetEnabledCORS(t *testing.T) {
	resp, body, err := req.Get("/version")
	require.NoError(t, err)
	defer body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	// TODO: @runcom incomplete tests, why old integration tests had this headers
	// and here none of the headers below are in the response?
	//c.Log(res.Header)
	//c.Assert(res.Header.Get("Access-Control-Allow-Origin"), check.Equals, "*")
	//c.Assert(res.Header.Get("Access-Control-Allow-Headers"), check.Equals, "Origin, X-Requested-With, Content-Type, Accept, X-Registry-Auth")
}

func TestClientVersionOldNotSupported(t *testing.T) {
	skip.If(t, testEnv.OSType != runtime.GOOS, "Daemon platform doesn't match test platform")

	skip.If(t, api.MinVersion == api.DefaultVersion, "API MinVersion==DefaultVersion")

	v := strings.Split(api.MinVersion, ".")
	vMinInt, err := strconv.Atoi(v[1])
	require.NoError(t, err)
	vMinInt--
	v[1] = strconv.Itoa(vMinInt)
	version := strings.Join(v, ".")

	resp, body, err := req.Get("/v" + version + "/version")
	require.NoError(t, err)
	defer body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)

	expected := fmt.Sprintf("client version %s is too old. Minimum supported API version is %s, please upgrade your client to a newer version", version, api.MinVersion)
	content, err := ioutil.ReadAll(body)
	require.NoError(t, err)
	assert.Contains(t, strings.TrimSpace(string(content)), expected)
}

func TestErrorJSON(t *testing.T) {
	resp, body, err := req.Post("/containers/create", req.JSONBody(struct{}{}))
	require.NoError(t, err)
	defer body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
	assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")

	b, err := req.ReadBody(body)
	require.NoError(t, err)
	var r types.ErrorResponse
	err = json.Unmarshal(b, &r)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(r.Message), "Config cannot be empty in order to create a container")
}

func TestErrorPlainText(t *testing.T) {
	// Windows requires API 1.25 or later. This test is validating a behaviour which was present
	// in v1.23, but changed in 1.24, hence not applicable on Windows. See apiVersionSupportsJSONErrors
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	resp, body, err := req.Post("/v1.23/containers/create", req.JSONBody(struct{}{}))
	require.NoError(t, err)
	defer body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")

	b, err := req.ReadBody(body)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(string(b)), "Config cannot be empty in order to create a container")
}

func TestErrorNotFoundJSON(t *testing.T) {
	// 404 is a different code path to normal errors, so test separately
	resp, body, err := req.Get("/notfound", req.JSON)
	require.NoError(t, err)
	defer body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)

	b, err := req.ReadBody(body)
	require.NoError(t, err)
	var r types.ErrorResponse
	err = json.Unmarshal(b, &r)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(r.Message), "page not found")
}

func TestErrorNotFoundPlainText(t *testing.T) {
	resp, body, err := req.Get("/v1.23/notfound", req.JSON)
	require.NoError(t, err)
	defer body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")

	b, err := req.ReadBody(body)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(string(b)), "page not found")
}
