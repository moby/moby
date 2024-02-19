package oci

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/oci"
	cdseccomp "github.com/containerd/containerd/pkg/seccomp"
	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/profiles/seccomp"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/entitlements/security"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var (
	cgroupNSOnce     sync.Once
	supportsCgroupNS bool
)

const (
	tracingSocketPath = "/dev/otel-grpc.sock"
)

func withProcessArgs(args ...string) oci.SpecOpts {
	return oci.WithProcessArgs(args...)
}

func generateMountOpts(resolvConf, hostsFile string) ([]oci.SpecOpts, error) {
	return []oci.SpecOpts{
		// https://github.com/moby/buildkit/issues/429
		withRemovedMount("/run"),
		withROBind(resolvConf, "/etc/resolv.conf"),
		withROBind(hostsFile, "/etc/hosts"),
		withCGroup(),
	}, nil
}

// generateSecurityOpts may affect mounts, so must be called after generateMountOpts
func generateSecurityOpts(mode pb.SecurityMode, apparmorProfile string, selinuxB bool) (opts []oci.SpecOpts, _ error) {
	if selinuxB && !selinux.GetEnabled() {
		return nil, errors.New("selinux is not available")
	}
	switch mode {
	case pb.SecurityMode_INSECURE:
		return []oci.SpecOpts{
			security.WithInsecureSpec(),
			oci.WithWriteableCgroupfs,
			oci.WithWriteableSysfs,
			func(_ context.Context, _ oci.Client, _ *containers.Container, s *oci.Spec) error {
				var err error
				if selinuxB {
					s.Process.SelinuxLabel, s.Linux.MountLabel, err = label.InitLabels([]string{"disable"})
				}
				return err
			},
		}, nil
	case pb.SecurityMode_SANDBOX:
		if cdseccomp.IsEnabled() {
			opts = append(opts, withDefaultProfile())
		}
		if apparmorProfile != "" {
			opts = append(opts, oci.WithApparmorProfile(apparmorProfile))
		}
		opts = append(opts, func(_ context.Context, _ oci.Client, _ *containers.Container, s *oci.Spec) error {
			var err error
			if selinuxB {
				s.Process.SelinuxLabel, s.Linux.MountLabel, err = label.InitLabels(nil)
			}
			return err
		})
		return opts, nil
	}
	return nil, nil
}

// generateProcessModeOpts may affect mounts, so must be called after generateMountOpts
func generateProcessModeOpts(mode ProcessMode) ([]oci.SpecOpts, error) {
	if mode == NoProcessSandbox {
		return []oci.SpecOpts{
			oci.WithHostNamespace(specs.PIDNamespace),
			withBoundProc(),
		}, nil
		// TODO(AkihiroSuda): Configure seccomp to disable ptrace (and prctl?) explicitly
	}
	return nil, nil
}

func generateIDmapOpts(idmap *idtools.IdentityMapping) ([]oci.SpecOpts, error) {
	if idmap == nil {
		return nil, nil
	}
	return []oci.SpecOpts{
		oci.WithUserNamespace(specMapping(idmap.UIDMaps), specMapping(idmap.GIDMaps)),
	}, nil
}

func specMapping(s []idtools.IDMap) []specs.LinuxIDMapping {
	var ids []specs.LinuxIDMapping
	for _, item := range s {
		ids = append(ids, specs.LinuxIDMapping{
			HostID:      uint32(item.HostID),
			ContainerID: uint32(item.ContainerID),
			Size:        uint32(item.Size),
		})
	}
	return ids
}

func generateRlimitOpts(ulimits []*pb.Ulimit) ([]oci.SpecOpts, error) {
	if len(ulimits) == 0 {
		return nil, nil
	}
	var rlimits []specs.POSIXRlimit
	for _, u := range ulimits {
		if u == nil {
			continue
		}
		rlimits = append(rlimits, specs.POSIXRlimit{
			Type: fmt.Sprintf("RLIMIT_%s", strings.ToUpper(u.Name)),
			Hard: uint64(u.Hard),
			Soft: uint64(u.Soft),
		})
	}
	return []oci.SpecOpts{
		func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
			s.Process.Rlimits = rlimits
			return nil
		},
	}, nil
}

// withDefaultProfile sets the default seccomp profile to the spec.
// Note: must follow the setting of process capabilities
func withDefaultProfile() oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		var err error
		s.Linux.Seccomp, err = seccomp.GetDefaultProfile(s)
		return err
	}
}

func withROBind(src, dest string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Mounts = append(s.Mounts, specs.Mount{
			Destination: dest,
			Type:        "bind",
			Source:      src,
			Options:     []string{"nosuid", "noexec", "nodev", "rbind", "ro"},
		})
		return nil
	}
}

func withCGroup() oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Mounts = append(s.Mounts, specs.Mount{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "cgroup",
			Options:     []string{"ro", "nosuid", "noexec", "nodev"},
		})
		return nil
	}
}

func withBoundProc() oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Mounts = removeMountsWithPrefix(s.Mounts, "/proc")
		procMount := specs.Mount{
			Destination: "/proc",
			Type:        "bind",
			Source:      "/proc",
			// NOTE: "rbind"+"ro" does not make /proc read-only recursively.
			// So we keep maskedPath and readonlyPaths (although not mandatory for rootless mode)
			Options: []string{"rbind"},
		}
		s.Mounts = append([]specs.Mount{procMount}, s.Mounts...)

		var maskedPaths []string
		for _, s := range s.Linux.MaskedPaths {
			if !hasPrefix(s, "/proc") {
				maskedPaths = append(maskedPaths, s)
			}
		}
		s.Linux.MaskedPaths = maskedPaths

		var readonlyPaths []string
		for _, s := range s.Linux.ReadonlyPaths {
			if !hasPrefix(s, "/proc") {
				readonlyPaths = append(readonlyPaths, s)
			}
		}
		s.Linux.ReadonlyPaths = readonlyPaths

		return nil
	}
}

func removeMountsWithPrefix(mounts []specs.Mount, prefixDir string) []specs.Mount {
	var ret []specs.Mount
	for _, m := range mounts {
		if !hasPrefix(m.Destination, prefixDir) {
			ret = append(ret, m)
		}
	}
	return ret
}

func getTracingSocketMount(socket string) *specs.Mount {
	return &specs.Mount{
		Destination: tracingSocketPath,
		Type:        "bind",
		Source:      socket,
		Options:     []string{"ro", "rbind"},
	}
}

func getTracingSocket() string {
	return fmt.Sprintf("unix://%s", tracingSocketPath)
}

func cgroupV2NamespaceSupported() bool {
	// Check if cgroups v2 namespaces are supported.  Trying to do cgroup
	// namespaces with cgroups v1 results in EINVAL when we encounter a
	// non-standard hierarchy.
	// See https://github.com/moby/buildkit/issues/4108
	cgroupNSOnce.Do(func() {
		if _, err := os.Stat("/proc/self/ns/cgroup"); os.IsNotExist(err) {
			return
		}
		if _, err := os.Stat("/sys/fs/cgroup/cgroup.subtree_control"); os.IsNotExist(err) {
			return
		}
		supportsCgroupNS = true
	})
	return supportsCgroupNS
}

func sub(m mount.Mount, subPath string) (mount.Mount, func() error, error) {
	var retries = 10
	root := m.Source
	for {
		src, err := fs.RootPath(root, subPath)
		if err != nil {
			return mount.Mount{}, nil, err
		}
		// similar to runc.WithProcfd
		fh, err := os.OpenFile(src, unix.O_PATH|unix.O_CLOEXEC, 0)
		if err != nil {
			return mount.Mount{}, nil, err
		}

		fdPath := "/proc/self/fd/" + strconv.Itoa(int(fh.Fd()))
		if resolved, err := os.Readlink(fdPath); err != nil {
			fh.Close()
			return mount.Mount{}, nil, err
		} else if resolved != src {
			retries--
			if retries <= 0 {
				fh.Close()
				return mount.Mount{}, nil, errors.Errorf("unable to safely resolve subpath %s", subPath)
			}
			fh.Close()
			continue
		}

		m.Source = fdPath
		lm := snapshot.LocalMounterWithMounts([]mount.Mount{m}, snapshot.ForceRemount())
		mp, err := lm.Mount()
		if err != nil {
			fh.Close()
			return mount.Mount{}, nil, err
		}
		m.Source = mp
		fh.Close() // release the fd, we don't need it anymore

		return m, lm.Unmount, nil
	}
}
