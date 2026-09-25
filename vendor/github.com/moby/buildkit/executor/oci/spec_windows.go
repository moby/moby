//go:build windows

package oci

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/continuity/fs"
	"github.com/moby/buildkit/solver/llbsolver/cdidevices"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/sys/user"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
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

func generateMountOpts(_, _ string) []oci.SpecOpts {
	return []oci.SpecOpts{
		withGetUserInfoMount(),
	}
}

// generateSecurityOpts may affect mounts, so must be called after generateMountOpts
func generateSecurityOpts(mode pb.SecurityMode, _ string, _ bool) ([]oci.SpecOpts, error) {
	if err := pb.ValidateSecurityMode(mode); err != nil {
		return nil, err
	}
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

func generateIDmapOpts(idmap *user.IdentityMapping) ([]oci.SpecOpts, error) {
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

	realSrc, release, err := resolveAndPinPath(src)
	if err != nil {
		return mount.Mount{}, nil, errors.Wrapf(err, "resolving and pinning mount source subpath %q", subPath)
	}
	if err := verifySubpathWithinRoot(m.Source, realSrc, subPath); err != nil {
		_ = release()
		return mount.Mount{}, nil, err
	}

	// Use the path of the object represented by the pinned handle. HCS can then
	// realize the mount without traversing attacker-controlled reparse points in
	// the original source path again.
	m.Source = mountSourcePath(realSrc)
	return m, release, nil
}

// resolveAndPinPath opens src while following reparse points and returns the
// normalized path of the object represented by that handle. GENERIC_READ makes
// the handle participate in share-access checks without requiring delete access
// to the source. Omitting delete sharing prevents the resolved directory from
// being renamed or deleted before HCS realizes the mount. Write sharing is still
// allowed so a writable cache mount can modify contents while the handle is alive.
func resolveAndPinPath(src string) (string, func() error, error) {
	pathPtr, err := windows.UTF16PtrFromString(src)
	if err != nil {
		return "", nil, err
	}
	h, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return "", nil, err
	}

	realSrc, err := finalPathNameByHandle(h)
	if err != nil {
		_ = windows.CloseHandle(h)
		return "", nil, err
	}
	return realSrc, func() error { return windows.CloseHandle(h) }, nil
}

func verifySubpathWithinRoot(root, realSrc, subPath string) error {
	realRoot, err := resolveFinalPath(root)
	if err != nil {
		return errors.Wrapf(err, "resolving mount root %q", root)
	}
	if !pathWithinRoot(realRoot, realSrc) {
		return errors.Errorf("mount source subpath %q resolves to %q which escapes the mount root %q", subPath, realSrc, realRoot)
	}
	return nil
}

// resolveFinalPath returns the real path of p with all reparse points followed.
func resolveFinalPath(p string) (string, error) {
	pathPtr, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return "", err
	}
	// FILE_FLAG_BACKUP_SEMANTICS allows opening a directory; omitting
	// FILE_FLAG_OPEN_REPARSE_POINT makes GetFinalPathNameByHandle follow links.
	h, err := windows.CreateFile(
		pathPtr,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	return finalPathNameByHandle(h)
}

func finalPathNameByHandle(h windows.Handle) (string, error) {
	// flags 0 == FILE_NAME_NORMALIZED | VOLUME_NAME_DOS.
	n := uint32(windows.MAX_PATH)
	for {
		buf := make([]uint16, n)
		got, err := windows.GetFinalPathNameByHandle(h, &buf[0], n, 0)
		if err != nil {
			return "", err
		}
		if got <= n {
			return windows.UTF16ToString(buf[:got]), nil
		}
		n = got
	}
}

func mountSourcePath(p string) string {
	if strings.HasPrefix(p, `\\?\UNC\`) {
		return `\\` + p[len(`\\?\UNC\`):]
	}
	if len(p) >= len(`\\?\C:\`) && p[:4] == `\\?\` && isDriveLetter(p[4]) && p[5:7] == `:\` {
		return p[4:]
	}
	return p
}

func isDriveLetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

// pathWithinRoot reports whether p is root or a descendant of root. Inputs are
// already-resolved real paths; filepath.Rel handles case-insensitivity and
// cross-volume paths on Windows.
func pathWithinRoot(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func generateLinuxResourceOpts(res *pb.LinuxResources) ([]oci.SpecOpts, error) {
	if res == nil {
		return nil, nil
	}
	return nil, errors.New("no support for Linux resource limits on Windows")
}

func generateCDIOpts(_ *cdidevices.Manager, devices []*pb.CDIDevice) ([]oci.SpecOpts, error) {
	if len(devices) == 0 {
		return nil, nil
	}
	// https://github.com/cncf-tags/container-device-interface/issues/28
	return nil, errors.New("no support for CDI on Windows")
}

func normalizeMountType(_ string) string {
	// HCS shim doesn't expect a named type
	// for the mount.
	return ""
}
