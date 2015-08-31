package main

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestApiOptionsRoute(c *check.C) {
	status, _, err := sockRequest("OPTIONS", "/", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)
}

func (s *DockerSuite) TestApiGetEnabledCors(c *check.C) {
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

func (s *DockerSuite) TestApiVersionStatusCode(c *check.C) {
	conn, err := sockConn(time.Duration(10 * time.Second))
	c.Assert(err, checker.IsNil)

	client := httputil.NewClientConn(conn, nil)
	defer client.Close()

	req, err := http.NewRequest("GET", "/v999.0/version", nil)
	c.Assert(err, checker.IsNil)
	req.Header.Set("User-Agent", "Docker-Client/999.0 (os)")

	res, err := client.Do(req)
	c.Assert(res.StatusCode, checker.Equals, http.StatusBadRequest)
}

func (s *DockerSuite) TestApiClientVersionNewerThanServer(c *check.C) {
	v := strings.Split(string(api.DefaultVersion), ".")
	vMinInt, err := strconv.Atoi(v[1])
	c.Assert(err, checker.IsNil)
	vMinInt++
	v[1] = strconv.Itoa(vMinInt)
	version := strings.Join(v, ".")

	status, body, err := sockRequest("GET", "/v"+version+"/version", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusBadRequest)
	c.Assert(len(string(body)), check.Not(checker.Equals), 0) // Expected not empty body
}

func (s *DockerSuite) TestApiClientVersionOldNotSupported(c *check.C) {
	v := strings.Split(string(api.MinVersion), ".")
	vMinInt, err := strconv.Atoi(v[1])
	c.Assert(err, checker.IsNil)
	vMinInt--
	v[1] = strconv.Itoa(vMinInt)
	version := strings.Join(v, ".")

	status, body, err := sockRequest("GET", "/v"+version+"/version", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusBadRequest)
	c.Assert(len(string(body)), checker.Not(check.Equals), 0) // Expected not empty body
}

func (s *DockerSuite) TestApiDockerApiVersion(c *check.C) {
	var svrVersion string

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			url := r.URL.Path
			svrVersion = url
		}))
	defer server.Close()

	// Test using the env var first
	cmd := exec.Command(dockerBinary, "-H="+server.URL[7:], "version")
	cmd.Env = append([]string{"DOCKER_API_VERSION=xxx"}, os.Environ()...)
	out, _, _ := runCommandWithOutput(cmd)

	c.Assert(svrVersion, check.Equals, "/vxxx/version")

	if !strings.Contains(out, "API version:  xxx") {
		c.Fatalf("Out didn't have 'xxx' for the API version, had:\n%s", out)
	}
}
