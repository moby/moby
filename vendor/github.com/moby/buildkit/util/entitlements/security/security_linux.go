package security

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/pkg/cap"
	"github.com/containerd/containerd/pkg/userns"
	"github.com/moby/buildkit/util/bklog"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// WithInsecureSpec sets spec with All capability.
func WithInsecureSpec() oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		addCaps, err := getAllCaps(ctx)
		if err != nil {
			return err
		}

		s.Process.Capabilities.Bounding = append(s.Process.Capabilities.Bounding, addCaps...)
		s.Process.Capabilities.Ambient = append(s.Process.Capabilities.Ambient, addCaps...)
		s.Process.Capabilities.Effective = append(s.Process.Capabilities.Effective, addCaps...)
		s.Process.Capabilities.Inheritable = append(s.Process.Capabilities.Inheritable, addCaps...)
		s.Process.Capabilities.Permitted = append(s.Process.Capabilities.Permitted, addCaps...)

		s.Linux.ReadonlyPaths = []string{}
		s.Linux.MaskedPaths = []string{}
		s.Process.ApparmorProfile = ""

		s.Linux.Resources.Devices = []specs.LinuxDeviceCgroup{
			{
				Allow:  true,
				Type:   "c",
				Access: "rwm",
			},
			{
				Allow:  true,
				Type:   "b",
				Access: "rwm",
			},
		}

		if !userns.RunningInUserNS() {
			// Devices automatically mounted on insecure mode
			s.Linux.Devices = append(s.Linux.Devices, []specs.LinuxDevice{
				// Writes to this come out as printk's, reads export the buffered printk records. (dmesg)
				{
					Path:  "/dev/kmsg",
					Type:  "c",
					Major: 1,
					Minor: 11,
				},
				// Cuse (character device in user-space)
				{
					Path:  "/dev/cuse",
					Type:  "c",
					Major: 10,
					Minor: 203,
				},
				// Fuse (virtual filesystem in user-space)
				{
					Path:  "/dev/fuse",
					Type:  "c",
					Major: 10,
					Minor: 229,
				},
				// Kernel-based virtual machine (hardware virtualization extensions)
				{
					Path:  "/dev/kvm",
					Type:  "c",
					Major: 10,
					Minor: 232,
				},
				// TAP/TUN network device
				{
					Path:  "/dev/net/tun",
					Type:  "c",
					Major: 10,
					Minor: 200,
				},
				// Loopback control device
				{
					Path:  "/dev/loop-control",
					Type:  "c",
					Major: 10,
					Minor: 237,
				},
			}...)

			loopID, err := getFreeLoopID()
			if err != nil {
				bklog.G(ctx).Debugf("failed to get next free loop device: %v", err)
			}

			for i := 0; i <= loopID+7; i++ {
				s.Linux.Devices = append(s.Linux.Devices, specs.LinuxDevice{
					Path:  fmt.Sprintf("/dev/loop%d", i),
					Type:  "b",
					Major: 7,
					Minor: int64(i),
				})
			}
		}

		return nil
	}
}

func getFreeLoopID() (int, error) {
	fd, err := os.OpenFile("/dev/loop-control", os.O_RDWR, 0644)
	if err != nil {
		return 0, err
	}
	defer fd.Close()

	const _LOOP_CTL_GET_FREE = 0x4C82 //nolint:revive
	r1, _, uerr := unix.Syscall(unix.SYS_IOCTL, fd.Fd(), _LOOP_CTL_GET_FREE, 0)
	if uerr == 0 {
		return int(r1), nil
	}
	return 0, errors.Errorf("error getting free loop device: %v", uerr)
}

var (
	currentCaps      []string
	currentCapsError error //nolint:errname
	currentCapsOnce  sync.Once
)

func getCurrentCaps() ([]string, error) {
	currentCapsOnce.Do(func() {
		currentCaps, currentCapsError = cap.Current()
	})

	return currentCaps, currentCapsError
}

func getAllCaps(ctx context.Context) ([]string, error) {
	availableCaps, err := getCurrentCaps()
	if err != nil {
		return nil, errors.Errorf("error getting current capabilities: %s", err)
	}

	// see if any of the base linux35Caps are not available to be granted
	// they are either not supported by the kernel or dropped at the process level
	for _, cap := range availableCaps {
		if _, exists := linux35Caps[cap]; !exists {
			bklog.G(ctx).Warnf("capability %s could not be granted for insecure mode", cap)
		}
	}

	return availableCaps, nil
}

// linux35Caps provides a list of capabilities available on Linux 3.5 kernel
var linux35Caps = map[string]struct{}{
	"CAP_FSETID":           {},
	"CAP_KILL":             {},
	"CAP_FOWNER":           {},
	"CAP_MKNOD":            {},
	"CAP_CHOWN":            {},
	"CAP_DAC_OVERRIDE":     {},
	"CAP_NET_RAW":          {},
	"CAP_SETGID":           {},
	"CAP_SETUID":           {},
	"CAP_SETPCAP":          {},
	"CAP_SETFCAP":          {},
	"CAP_NET_BIND_SERVICE": {},
	"CAP_SYS_CHROOT":       {},
	"CAP_AUDIT_WRITE":      {},
	"CAP_MAC_ADMIN":        {},
	"CAP_MAC_OVERRIDE":     {},
	"CAP_DAC_READ_SEARCH":  {},
	"CAP_SYS_PTRACE":       {},
	"CAP_SYS_MODULE":       {},
	"CAP_SYSLOG":           {},
	"CAP_SYS_RAWIO":        {},
	"CAP_SYS_ADMIN":        {},
	"CAP_LINUX_IMMUTABLE":  {},
	"CAP_SYS_BOOT":         {},
	"CAP_SYS_NICE":         {},
	"CAP_SYS_PACCT":        {},
	"CAP_SYS_TTY_CONFIG":   {},
	"CAP_SYS_TIME":         {},
	"CAP_WAKE_ALARM":       {},
	"CAP_AUDIT_READ":       {},
	"CAP_AUDIT_CONTROL":    {},
	"CAP_SYS_RESOURCE":     {},
	"CAP_BLOCK_SUSPEND":    {},
	"CAP_IPC_LOCK":         {},
	"CAP_IPC_OWNER":        {},
	"CAP_LEASE":            {},
	"CAP_NET_ADMIN":        {},
	"CAP_NET_BROADCAST":    {},
}
