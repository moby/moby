package daemon // import "github.com/docker/docker/daemon"

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/libnetwork/resolvconf"
	"github.com/moby/sys/mount"
	"github.com/moby/sys/mountinfo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
func (daemon *Daemon) cleanupMounts() error {
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

	unmountFile := getUnmountOnShutdownPath(daemon.configStore)
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
	patterns = append(patterns, "overlay2/"+id+"/merged$", "aufs/mnt/"+id+"$", "overlay/"+id+"/merged$", "zfs/graph/"+id+"$")
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
