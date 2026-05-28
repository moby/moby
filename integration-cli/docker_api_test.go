package main

import (
	"context"
	"testing"
)

type DockerAPISuite struct {
	ds *DockerSuite
}

func (s *DockerAPISuite) TearDownTest(ctx context.Context, t *testing.T) {
	s.ds.TearDownTest(ctx, t)
}

func (s *DockerAPISuite) OnTimeout(t *testing.T) {
	s.ds.OnTimeout(t)
}
