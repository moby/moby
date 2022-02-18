package main

import (
	"net/http/httptest"

	"github.com/moby/moby/integration-cli/daemon"
)

type DockerNetworkSuite struct {
	server *httptest.Server
	ds     *DockerSuite
	d      *daemon.Daemon
}
