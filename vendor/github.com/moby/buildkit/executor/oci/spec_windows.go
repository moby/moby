//go:build windows
// +build windows

package oci

import (
	"fmt"
	"path/filepath"

	"github.com/containerd/containerd/oci"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/solver/pb"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

const (
	tracingSocketPath = "//./pipe/otel-grpc"
)

func generateMountOpts(resolvConf, hostsFile string) ([]oci.SpecOpts, error) {
	return nil, nil
}

// generateSecurityOpts may affect mounts, so must be called after generateMountOpts
func generateSecurityOpts(mode pb.SecurityMode, apparmorProfile string, selinuxB bool) ([]oci.SpecOpts, error) {
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

func getTracingSocketMount(socket string) specs.Mount {
	return specs.Mount{
		Destination: filepath.FromSlash(tracingSocketPath),
		Source:      socket,
		Options:     []string{"ro"},
	}
}

func getTracingSocket() string {
	return fmt.Sprintf("npipe://%s", filepath.ToSlash(tracingSocketPath))
}

func cgroupNamespaceSupported() bool {
	return false
}
