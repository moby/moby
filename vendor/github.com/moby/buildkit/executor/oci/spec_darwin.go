package oci

import (
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/solver/pb"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func withProcessArgs(args ...string) oci.SpecOpts {
	return oci.WithProcessArgs(args...)
}

func generateMountOpts(_, _ string) []oci.SpecOpts {
	return nil
}

func generateSecurityOpts(mode pb.SecurityMode, _ string, _ bool) ([]oci.SpecOpts, error) {
	return nil, nil
}

func generateProcessModeOpts(mode ProcessMode) ([]oci.SpecOpts, error) {
	return nil, nil
}

func generateIDmapOpts(idmap *idtools.IdentityMapping) ([]oci.SpecOpts, error) {
	if idmap == nil {
		return nil, nil
	}
	return nil, errors.New("no support for IdentityMapping on Darwin")
}

func generateRlimitOpts(ulimits []*pb.Ulimit) ([]oci.SpecOpts, error) {
	if len(ulimits) == 0 {
		return nil, nil
	}
	return nil, errors.New("no support for POSIXRlimit on Darwin")
}

// tracing is not implemented on Darwin
func getTracingSocketMount(_ string) *specs.Mount {
	return nil
}

// tracing is not implemented on Darwin
func getTracingSocket() string {
	return ""
}

func cgroupV2NamespaceSupported() bool {
	return false
}

func sub(m mount.Mount, subPath string) (mount.Mount, func() error, error) {
	src, err := fs.RootPath(m.Source, subPath)
	if err != nil {
		return mount.Mount{}, nil, err
	}
	m.Source = src
	return m, func() error { return nil }, nil
}
