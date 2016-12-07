package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"

	"github.com/docker/docker/api"
	"github.com/docker/docker/pkg/integration"
	"github.com/docker/docker/pkg/integration/checker"
	icmd "github.com/docker/docker/pkg/integration/cmd"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestAPIOptionsRoute(c *check.C) {
	status, _, err := sockRequest("OPTIONS", "/", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
}

func (s *DockerSuite) TestAPIGetEnabledCORS(c *check.C) {
	res, body, err := sockRequestRaw("GET", "/version", nil, "")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)
	body.Close()
	// TODO: @runcom incomplete tests, why old integration tests had this headers
	// and here none of the headers below are in the response?
	//c.Log(res.Header)
	//c.Assert(res.Header.Get("Access-Control-Allow-Origin"), check.Equals, "*")
	//c.Assert(res.Header.Get("Access-Control-Allow-Headers"), check.Equals, "Origin, X-Requested-With, Content-Type, Accept, X-Registry-Auth")
}

func (s *DockerSuite) TestAPIClientVersionOldNotSupported(c *check.C) {
	if daemonPlatform != runtime.GOOS {
		c.Skip("Daemon platform doesn't match test platform")
	}
	if api.MinVersion == api.DefaultVersion {
		c.Skip("API MinVersion==DefaultVersion")
	}
	v := strings.Split(api.MinVersion, ".")
	vMinInt, err := strconv.Atoi(v[1])
	c.Assert(err, checker.IsNil)
	vMinInt--
	v[1] = strconv.Itoa(vMinInt)
	version := strings.Join(v, ".")

	status, body, err := sockRequest("GET", "/v"+version+"/version", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusBadRequest)
	expected := fmt.Sprintf("client version %s is too old. Minimum supported API version is %s, please upgrade your client to a newer version", version, api.MinVersion)
	c.Assert(strings.TrimSpace(string(body)), checker.Contains, expected)
}

func (s *DockerSuite) TestAPIDockerAPIVersion(c *check.C) {
	var svrVersion string

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("API-Version", api.DefaultVersion)
			url := r.URL.Path
			svrVersion = url
		}))
	defer server.Close()

	// Test using the env var first
	result := icmd.RunCmd(icmd.Cmd{
		Command: binaryWithArgs("-H="+server.URL[7:], "version"),
		Env:     appendBaseEnv(false, "DOCKER_API_VERSION=xxx"),
	})
	c.Assert(result, icmd.Matches, icmd.Expected{Out: "API version:  xxx", ExitCode: 1})
	c.Assert(svrVersion, check.Equals, "/vxxx/version", check.Commentf("%s", result.Compare(icmd.Success)))
}

func (s *DockerSuite) TestAPIErrorJSON(c *check.C) {
	httpResp, body, err := sockRequestRaw("POST", "/containers/create", strings.NewReader(`{}`), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(httpResp.StatusCode, checker.Equals, http.StatusInternalServerError)
	c.Assert(httpResp.Header.Get("Content-Type"), checker.Equals, "application/json")
	b, err := integration.ReadBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(getErrorMessage(c, b), checker.Equals, "Config cannot be empty in order to create a container")
}

func (s *DockerSuite) TestAPIErrorPlainText(c *check.C) {
	// Windows requires API 1.25 or later. This test is validating a behaviour which was present
	// in v1.23, but changed in 1.24, hence not applicable on Windows. See apiVersionSupportsJSONErrors
	testRequires(c, DaemonIsLinux)
	httpResp, body, err := sockRequestRaw("POST", "/v1.23/containers/create", strings.NewReader(`{}`), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(httpResp.StatusCode, checker.Equals, http.StatusInternalServerError)
	c.Assert(httpResp.Header.Get("Content-Type"), checker.Contains, "text/plain")
	b, err := integration.ReadBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(string(b)), checker.Equals, "Config cannot be empty in order to create a container")
}

func (s *DockerSuite) TestAPIErrorNotFoundJSON(c *check.C) {
	// 404 is a different code path to normal errors, so test separately
	httpResp, body, err := sockRequestRaw("GET", "/notfound", nil, "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(httpResp.StatusCode, checker.Equals, http.StatusNotFound)
	c.Assert(httpResp.Header.Get("Content-Type"), checker.Equals, "application/json")
	b, err := integration.ReadBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(getErrorMessage(c, b), checker.Equals, "page not found")
}

func (s *DockerSuite) TestAPIErrorNotFoundPlainText(c *check.C) {
	httpResp, body, err := sockRequestRaw("GET", "/v1.23/notfound", nil, "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(httpResp.StatusCode, checker.Equals, http.StatusNotFound)
	c.Assert(httpResp.Header.Get("Content-Type"), checker.Contains, "text/plain")
	b, err := integration.ReadBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(string(b)), checker.Equals, "page not found")
}
