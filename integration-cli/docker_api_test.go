package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api"
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

func (s *DockerSuite) TestAPIVersionStatusCode(c *check.C) {
	conn, err := sockConn(time.Duration(10*time.Second), "")
	c.Assert(err, checker.IsNil)

	client := httputil.NewClientConn(conn, nil)
	defer client.Close()

	req, err := http.NewRequest("GET", "/v999.0/version", nil)
	c.Assert(err, checker.IsNil)
	req.Header.Set("User-Agent", "Docker-Client/999.0 (os)")

	res, err := client.Do(req)
	c.Assert(res.StatusCode, checker.Equals, http.StatusBadRequest)
}

func (s *DockerSuite) TestAPIClientVersionNewerThanServer(c *check.C) {
	v := strings.Split(api.DefaultVersion, ".")
	vMinInt, err := strconv.Atoi(v[1])
	c.Assert(err, checker.IsNil)
	vMinInt++
	v[1] = strconv.Itoa(vMinInt)
	version := strings.Join(v, ".")

	status, body, err := sockRequest("GET", "/v"+version+"/version", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusBadRequest)
	expected := fmt.Sprintf("client is newer than server (client API version: %s, server API version: %s)", version, api.DefaultVersion)
	c.Assert(getErrorMessage(c, body), checker.Equals, expected)
}

func (s *DockerSuite) TestAPIClientVersionOldNotSupported(c *check.C) {
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
	c.Assert(strings.TrimSpace(string(body)), checker.Equals, expected)
}

func (s *DockerSuite) TestAPIDockerAPIVersion(c *check.C) {
	var svrVersion string

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
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
	b, err := readBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(getErrorMessage(c, b), checker.Equals, "Config cannot be empty in order to create a container")
}

func (s *DockerSuite) TestAPIErrorPlainText(c *check.C) {
	httpResp, body, err := sockRequestRaw("POST", "/v1.23/containers/create", strings.NewReader(`{}`), "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(httpResp.StatusCode, checker.Equals, http.StatusInternalServerError)
	c.Assert(httpResp.Header.Get("Content-Type"), checker.Contains, "text/plain")
	b, err := readBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(string(b)), checker.Equals, "Config cannot be empty in order to create a container")
}

func (s *DockerSuite) TestAPIErrorNotFoundJSON(c *check.C) {
	// 404 is a different code path to normal errors, so test separately
	httpResp, body, err := sockRequestRaw("GET", "/notfound", nil, "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(httpResp.StatusCode, checker.Equals, http.StatusNotFound)
	c.Assert(httpResp.Header.Get("Content-Type"), checker.Equals, "application/json")
	b, err := readBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(getErrorMessage(c, b), checker.Equals, "page not found")
}

func (s *DockerSuite) TestAPIErrorNotFoundPlainText(c *check.C) {
	httpResp, body, err := sockRequestRaw("GET", "/v1.23/notfound", nil, "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(httpResp.StatusCode, checker.Equals, http.StatusNotFound)
	c.Assert(httpResp.Header.Get("Content-Type"), checker.Contains, "text/plain")
	b, err := readBody(body)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(string(b)), checker.Equals, "page not found")
}
