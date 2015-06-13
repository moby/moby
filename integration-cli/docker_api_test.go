package main

import (
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestApiOptionsRoute(c *check.C) {
	status, _, err := sockRequest("OPTIONS", "/", nil)
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)
}

func (s *DockerSuite) TestApiGetEnabledCors(c *check.C) {
	res, body, err := sockRequestRaw("GET", "/version", nil, "")
	body.Close()
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusOK)
	// TODO: @runcom incomplete tests, why old integration tests had this headers
	// and here none of the headers below are in the response?
	//c.Log(res.Header)
	//c.Assert(res.Header.Get("Access-Control-Allow-Origin"), check.Equals, "*")
	//c.Assert(res.Header.Get("Access-Control-Allow-Headers"), check.Equals, "Origin, X-Requested-With, Content-Type, Accept, X-Registry-Auth")
}

func (s *DockerSuite) TestVersionStatusCode(c *check.C) {
	conn, err := sockConn(time.Duration(10 * time.Second))
	c.Assert(err, check.IsNil)

	client := httputil.NewClientConn(conn, nil)
	defer client.Close()

	req, err := http.NewRequest("GET", "/v999.0/version", nil)
	c.Assert(err, check.IsNil)
	req.Header.Set("User-Agent", "Docker-Client/999.0 (os)")

	res, err := client.Do(req)
	c.Assert(res.StatusCode, check.Equals, http.StatusBadRequest)
}
