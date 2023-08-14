package main

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/integration-cli/daemon"
)

type DockerCLINetworkSuite struct {
	ds *DockerSuite
}

func (s *DockerCLINetworkSuite) TearDownTest(ctx context.Context, c *testing.T) {
	s.ds.TearDownTest(ctx, c)
}

func (s *DockerCLINetworkSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

type DockerNetworkSuite struct {
	server *httptest.Server
	ds     *DockerSuite
	d      *daemon.Daemon
}
