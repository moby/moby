package main

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/moby/moby/v2/integration-cli/daemon"
)

type DockerCLINetworkSuite struct {
	ds *DockerSuite
}

func (s *DockerCLINetworkSuite) TearDownTest(ctx context.Context, t *testing.T) {
	s.ds.TearDownTest(ctx, t)
}

func (s *DockerCLINetworkSuite) OnTimeout(t *testing.T) {
	s.ds.OnTimeout(t)
}

type DockerNetworkSuite struct {
	server *httptest.Server
	ds     *DockerSuite
	d      *daemon.Daemon
}
