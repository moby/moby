package oci

import (
	"context"

	"github.com/containerd/containerd/containers"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// SpecOpts sets spec specific information to a newly generated OCI spec
type SpecOpts func(context.Context, Client, *containers.Container, *specs.Spec) error

// WithProcessArgs replaces the args on the generated spec
func WithProcessArgs(args ...string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		s.Process.Args = args
		return nil
	}
}

// WithProcessCwd replaces the current working directory on the generated spec
func WithProcessCwd(cwd string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		s.Process.Cwd = cwd
		return nil
	}
}

// WithHostname sets the container's hostname
func WithHostname(name string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		s.Hostname = name
		return nil
	}
}
