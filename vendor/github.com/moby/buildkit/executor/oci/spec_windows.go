//go:build windows
// +build windows

package oci

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/solver/pb"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

const (
	tracingSocketPath = "//./pipe/otel-grpc"
)

func withProcessArgs(args ...string) oci.SpecOpts {
	cmdLine := strings.Join(args, " ")
	// This will set Args to nil and properly set the CommandLine option
	// in the spec. On Windows we need to use CommandLine instead of Args.
	return oci.WithProcessCommandLine(cmdLine)
}

func withGetUserInfoMount() oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		execPath, err := os.Executable()
		if err != nil {
			return errors.Wrap(err, "getting executable path")
		}
		// The buildkit binary registers a re-exec function that is invoked when called with
		// get-user-info as the name. We mount the binary as read-only inside the container. This
		// spares us from having to ship a separate binary just for this purpose. The container does
		// not share any state with the running buildkit daemon. In this scenario, we use the re-exec
		// functionality to simulate a multi-call binary.
		s.Mounts = append(s.Mounts, specs.Mount{
			Destination: "C:\\Windows\\System32\\get-user-info.exe",
			Source:      execPath,
			Options:     []string{"ro"},
		})
		return nil
	}
}

func generateMountOpts(_, _ string) ([]oci.SpecOpts, error) {
	return []oci.SpecOpts{
		withGetUserInfoMount(),
	}, nil
}

// generateSecurityOpts may affect mounts, so must be called after generateMountOpts
func generateSecurityOpts(mode pb.SecurityMode, _ string, _ bool) ([]oci.SpecOpts, error) {
	if mode == pb.SecurityMode_INSECURE {
		return nil, errors.New("no support for running in insecure mode on Windows")
	}
	return nil, nil
}

// generateProcessModeOpts may affect mounts, so must be called after generateMountOpts
func generateProcessModeOpts(mode ProcessMode) ([]oci.SpecOpts, error) {
	if mode == NoProcessSandbox {
		return nil, errors.New("no support for NoProcessSandbox on Windows")
	}
	return nil, nil
}

func generateIDmapOpts(idmap *idtools.IdentityMapping) ([]oci.SpecOpts, error) {
	if idmap == nil {
		return nil, nil
	}
	return nil, errors.New("no support for IdentityMapping on Windows")
}

func generateRlimitOpts(ulimits []*pb.Ulimit) ([]oci.SpecOpts, error) {
	if len(ulimits) == 0 {
		return nil, nil
	}
	return nil, errors.New("no support for POSIXRlimit on Windows")
}

func getTracingSocketMount(socket string) *specs.Mount {
	return &specs.Mount{
		Destination: filepath.FromSlash(tracingSocketPath),
		Source:      socket,
		Options:     []string{"ro"},
	}
}

func getTracingSocket() string {
	return fmt.Sprintf("npipe://%s", filepath.ToSlash(tracingSocketPath))
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
