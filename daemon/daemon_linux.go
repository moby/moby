package daemon // import "github.com/docker/docker/daemon"

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/resolvconf"
	"github.com/moby/sys/mount"
	"github.com/moby/sys/mountinfo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// On Linux, plugins use a static path for storing execution state,
// instead of deriving path from daemon's exec-root. This is because
// plugin socket files are created here and they cannot exceed max
// path length of 108 bytes.
func getPluginExecRoot(_ *config.Config) string {
	return "/run/docker/plugins"
}

func (daemon *Daemon) cleanupMountsByID(id string) error {
	logrus.Debugf("Cleaning up old mountid %s: start.", id)
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return err
	}
	defer f.Close()

	return daemon.cleanupMountsFromReaderByID(f, id, mount.Unmount)
}

func (daemon *Daemon) cleanupMountsFromReaderByID(reader io.Reader, id string, unmount func(target string) error) error {
	if daemon.root == "" {
		return nil
	}
	var errs []string

	regexps := getCleanPatterns(id)
	sc := bufio.NewScanner(reader)
	for sc.Scan() {
		if fields := strings.Fields(sc.Text()); len(fields) > 4 {
			if mnt := fields[4]; strings.HasPrefix(mnt, daemon.root) {
				for _, p := range regexps {
					if p.MatchString(mnt) {
						if err := unmount(mnt); err != nil {
							logrus.Error(err)
							errs = append(errs, err.Error())
						}
					}
				}
			}
		}
	}

	if err := sc.Err(); err != nil {
		return err
	}

	if len(errs) > 0 {
		return fmt.Errorf("Error cleaning up mounts:\n%v", strings.Join(errs, "\n"))
	}

	logrus.Debugf("Cleaning up old mountid %v: done.", id)
	return nil
}

// cleanupMounts umounts used by container resources and the daemon root mount
func (daemon *Daemon) cleanupMounts(cfg *config.Config) error {
	if err := daemon.cleanupMountsByID(""); err != nil {
		return err
	}

	info, err := mountinfo.GetMounts(mountinfo.SingleEntryFilter(daemon.root))
	if err != nil {
		return errors.Wrap(err, "error reading mount table for cleanup")
	}

	if len(info) < 1 {
		// no mount found, we're done here
		return nil
	}

	// `info.Root` here is the root mountpoint of the passed in path (`daemon.root`).
	// The ony cases that need to be cleaned up is when the daemon has performed a
	//   `mount --bind /daemon/root /daemon/root && mount --make-shared /daemon/root`
	// This is only done when the daemon is started up and `/daemon/root` is not
	// already on a shared mountpoint.
	if !shouldUnmountRoot(daemon.root, info[0]) {
		return nil
	}

	unmountFile := getUnmountOnShutdownPath(cfg)
	if _, err := os.Stat(unmountFile); err != nil {
		return nil
	}

	logrus.WithField("mountpoint", daemon.root).Debug("unmounting daemon root")
	if err := mount.Unmount(daemon.root); err != nil {
		return err
	}
	return os.Remove(unmountFile)
}

func getCleanPatterns(id string) (regexps []*regexp.Regexp) {
	var patterns []string
	if id == "" {
		id = "[0-9a-f]{64}"
		patterns = append(patterns, "containers/"+id+"/mounts/shm", "containers/"+id+"/shm")
	}
	patterns = append(patterns, "overlay2/"+id+"/merged$", "zfs/graph/"+id+"$")
	for _, p := range patterns {
		r, err := regexp.Compile(p)
		if err == nil {
			regexps = append(regexps, r)
		}
	}
	return
}

func shouldUnmountRoot(root string, info *mountinfo.Info) bool {
	if !strings.HasSuffix(root, info.Root) {
		return false
	}
	return hasMountInfoOption(info.Optional, sharedPropagationOption)
}

// setupResolvConf sets the appropriate resolv.conf file if not specified
// When systemd-resolved is running the default /etc/resolv.conf points to
// localhost. In this case fetch the alternative config file that is in a
// different path so that containers can use it
// In all the other cases fallback to the default one
func setupResolvConf(config *config.Config) {
	if config.ResolvConf != "" {
		return
	}
	config.ResolvConf = resolvconf.Path()
}

// ifaceAddrs returns the IPv4 and IPv6 addresses assigned to the network
// interface with name linkName.
//
// No error is returned if the named interface does not exist.
func ifaceAddrs(linkName string) (v4, v6 []*net.IPNet, err error) {
	nl := ns.NlHandle()
	link, err := nl.LinkByName(linkName)
	if err != nil {
		if !errors.As(err, new(netlink.LinkNotFoundError)) {
			return nil, nil, err
		}
		return nil, nil, nil
	}

	get := func(family int) ([]*net.IPNet, error) {
		addrs, err := nl.AddrList(link, family)
		if err != nil {
			return nil, err
		}

		ipnets := make([]*net.IPNet, len(addrs))
		for i := range addrs {
			ipnets[i] = addrs[i].IPNet
		}
		return ipnets, nil
	}

	v4, err = get(netlink.FAMILY_V4)
	if err != nil {
		return nil, nil, err
	}
	v6, err = get(netlink.FAMILY_V6)
	if err != nil {
		return nil, nil, err
	}
	return v4, v6, nil
}

var (
	kernelSupportsRROOnce sync.Once
	kernelSupportsRROErr  error
)

func kernelSupportsRecursivelyReadOnly() error {
	fn := func() error {
		tmpMnt, err := os.MkdirTemp("", "moby-detect-rro")
		if err != nil {
			return fmt.Errorf("failed to create a temp directory: %w", err)
		}
		for {
			err = unix.Mount("", tmpMnt, "tmpfs", 0, "")
			if !errors.Is(err, unix.EINTR) {
				break
			}
		}
		if err != nil {
			return fmt.Errorf("failed to mount tmpfs on %q: %w", tmpMnt, err)
		}
		defer func() {
			var umErr error
			for {
				umErr = unix.Unmount(tmpMnt, 0)
				if !errors.Is(umErr, unix.EINTR) {
					break
				}
			}
			if umErr != nil {
				logrus.WithError(umErr).Warnf("Failed to unmount %q", tmpMnt)
			}
		}()
		attr := &unix.MountAttr{
			Attr_set: unix.MOUNT_ATTR_RDONLY,
		}
		for {
			err = unix.MountSetattr(-1, tmpMnt, unix.AT_RECURSIVE, attr)
			if !errors.Is(err, unix.EINTR) {
				break
			}
		}
		// ENOSYS on kernel < 5.12
		if err != nil {
			return fmt.Errorf("failed to call mount_setattr: %w", err)
		}
		return nil
	}

	kernelSupportsRROOnce.Do(func() {
		kernelSupportsRROErr = fn()
	})
	return kernelSupportsRROErr
}

func supportsRecursivelyReadOnly(cfg *config.Config, runtime string) error {
	if err := kernelSupportsRecursivelyReadOnly(); err != nil {
		return fmt.Errorf("rro is not supported: %w (kernel is older than 5.12?)", err)
	}
	if runtime == "" {
		runtime = cfg.DefaultRuntime
	}
	rt := cfg.GetRuntime(runtime)
	if rt.Features == nil {
		return fmt.Errorf("rro is not supported by runtime %q: OCI features struct is not available", runtime)
	}
	for _, s := range rt.Features.MountOptions {
		if s == "rro" {
			return nil
		}
	}
	return fmt.Errorf("rro is not supported by runtime %q", runtime)
}
