package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/go-check/check"
)

func makefile(contents string) (string, func(), error) {
	cleanup := func() {

	}

	f, err := ioutil.TempFile(".", "tmp")
	if err != nil {
		return "", cleanup, err
	}
	err = ioutil.WriteFile(f.Name(), []byte(contents), os.ModePerm)
	if err != nil {
		return "", cleanup, err
	}

	cleanup = func() {
		err := os.Remove(f.Name())
		if err != nil {
			fmt.Println("Error removing tmpfile")
		}
	}
	return f.Name(), cleanup, nil

}

// TestV2Only ensures that a daemon in v2-only mode does not
// attempt to contact any v1 registry endpoints.
func (s *DockerRegistrySuite) TestV2Only(c *check.C) {
	reg, err := newTestRegistry(c)
	c.Assert(err, check.IsNil)

	reg.registerHandler("/v2/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	reg.registerHandler("/v1/.*", func(w http.ResponseWriter, r *http.Request) {
		c.Fatal("V1 registry contacted")
	})

	repoName := fmt.Sprintf("%s/busybox", reg.hostport)

	err = s.d.Start("--insecure-registry", reg.hostport, "--disable-legacy-registry=true")
	c.Assert(err, check.IsNil)

	dockerfileName, cleanup, err := makefile(fmt.Sprintf("FROM %s/busybox", reg.hostport))
	c.Assert(err, check.IsNil, check.Commentf("Unable to create test dockerfile"))
	defer cleanup()

	s.d.Cmd("build", "--file", dockerfileName, ".")

	s.d.Cmd("run", repoName)
	s.d.Cmd("login", "-u", "richard", "-p", "testtest", "-e", "testuser@testdomain.com", reg.hostport)
	s.d.Cmd("tag", "busybox", repoName)
	s.d.Cmd("push", repoName)
	s.d.Cmd("pull", repoName)
}

// TestV1 starts a daemon in 'normal' mode
// and ensure v1 endpoints are hit for the following operations:
// login, push, pull, build & run
func (s *DockerRegistrySuite) TestV1(c *check.C) {
	reg, err := newTestRegistry(c)
	c.Assert(err, check.IsNil)

	v2Pings := 0
	reg.registerHandler("/v2/", func(w http.ResponseWriter, r *http.Request) {
		v2Pings++
		// V2 ping 404 causes fallback to v1
		w.WriteHeader(404)
	})

	v1Pings := 0
	reg.registerHandler("/v1/_ping", func(w http.ResponseWriter, r *http.Request) {
		v1Pings++
	})

	v1Logins := 0
	reg.registerHandler("/v1/users/", func(w http.ResponseWriter, r *http.Request) {
		v1Logins++
	})

	v1Repo := 0
	reg.registerHandler("/v1/repositories/busybox/", func(w http.ResponseWriter, r *http.Request) {
		v1Repo++
	})

	reg.registerHandler("/v1/repositories/busybox/images", func(w http.ResponseWriter, r *http.Request) {
		v1Repo++
	})

	err = s.d.Start("--insecure-registry", reg.hostport, "--disable-legacy-registry=false")
	c.Assert(err, check.IsNil)

	dockerfileName, cleanup, err := makefile(fmt.Sprintf("FROM %s/busybox", reg.hostport))
	c.Assert(err, check.IsNil, check.Commentf("Unable to create test dockerfile"))
	defer cleanup()

	s.d.Cmd("build", "--file", dockerfileName, ".")
	c.Assert(v1Repo, check.Not(check.Equals), 0, check.Commentf("Expected v1 repository access after build"))

	repoName := fmt.Sprintf("%s/busybox", reg.hostport)
	s.d.Cmd("run", repoName)
	c.Assert(v1Repo, check.Not(check.Equals), 1, check.Commentf("Expected v1 repository access after run"))

	s.d.Cmd("login", "-u", "richard", "-p", "testtest", reg.hostport)
	c.Assert(v1Logins, check.Not(check.Equals), 0, check.Commentf("Expected v1 login attempt"))

	s.d.Cmd("tag", "busybox", repoName)
	s.d.Cmd("push", repoName)

	c.Assert(v1Repo, check.Equals, 2)
	c.Assert(v1Pings, check.Equals, 1)

	s.d.Cmd("pull", repoName)
	c.Assert(v1Repo, check.Equals, 3, check.Commentf("Expected v1 repository access after pull"))
}
