package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/go-cni"
	"github.com/docker/docker/internal/test"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"gotest.tools/assert"
	"gotest.tools/icmd"
)

var (
	nsenterOnce sync.Once
	nsenterPath string
)

func cleanupNetworkNamespace(t testingT, execRoot string) {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	// Cleanup network namespaces in the exec root of this
	// daemon because this exec root is specific to this
	// daemon instance and has no chance of getting
	// cleaned up when a new daemon is instantiated with a
	// new exec root.
	netnsPath := filepath.Join(execRoot, "netns")
	filepath.Walk(netnsPath, func(path string, info os.FileInfo, err error) error {
		if err := unix.Unmount(path, unix.MNT_DETACH); err != nil && err != unix.EINVAL && err != unix.ENOENT {
			t.Logf("unmount of %s failed: %v", path, err)
		}
		os.Remove(path)
		return nil
	})
}

func (d *Daemon) configureNetNS() (retErr error) {
	if d.cniConfig != nil {
		return nil
	}
	started := time.Now()
	defer func() {
		d.log.Logf("[%s] configureNetNS duration: %f", d.id, time.Since(started).Seconds())
	}()
	defer func() {
		if retErr != nil {
			retErr = errors.Wrap(retErr, "error configuring daemon network namespace")
		}
	}()

	if err := ensureDaemonNetworks(); err != nil {
		return err
	}

	cmd := exec.Command("ip", "netns", "add", d.id)
	d.log.Logf("[%s] creating network namespace", d.id)
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrap(err, string(out))
	}

	delCmd := exec.Command("ip", "netns", "del", d.id)
	defer func() {
		if retErr != nil {
			delCmd.Run()
		}
	}()

	nsPath := d.netnsPath()
	d.log.Logf("[%s] setting up cni networking", d.id)
	cniConfig, err := daemonNetworks.Setup(d.id, nsPath)
	if err != nil {
		return errors.Wrap(err, "error setting up CNI network")
	}
	d.cniConfig = cniConfig

CNIInterfaces:
	for _, i := range cniConfig.Interfaces {
		for _, cfg := range i.IPConfigs {
			if cfg.IP.IsLoopback() {
				continue
			}
			d.swarmListenAddr = cfg.IP.String()
			break CNIInterfaces
		}
	}

	d.cleanupHandlers = append(d.cleanupHandlers, func(t testingT) {
		d.log.Logf("[%s] cleaning up cni networking", d.id)
		if err := daemonNetworks.Remove(d.id, nsPath); err != nil {
			t.Logf("[%s] error removing cni network:", d.id, err.Error())
		}

		d.log.Logf("[%s] removing network namespace", d.id)
		if out, err := delCmd.CombinedOutput(); err != nil {
			t.Logf("[%s] error deleting network namespace: %s", d.id, string(out))
		}

		d.cniConfig = nil
	})
	return nil
}

func (d *Daemon) ensureNetworking(t assert.TestingT) {
	if th, ok := t.(test.HelperT); ok {
		th.Helper()
	}
	assert.NilError(t, d.configureNetNS())
}

func (d *Daemon) configureCmd() error {
	if err := d.configureNetNS(); err != nil {
		return err
	}
	if err := ensureNsenter(); err != nil {
		return err
	}

	d.cmd.Args = append([]string{"nsenter", "--net=" + d.netnsPath(), d.cmd.Path}, d.cmd.Args[1:]...)
	d.cmd.Path = nsenterPath
	d.cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
	return nil
}

func (d *Daemon) netnsPath() string {
	return "/run/netns/" + d.id
}

func ensureNsenter() error {
	var err error
	nsenterOnce.Do(func() {
		nsenterPath, err = exec.LookPath("nsenter")
	})
	return errors.Wrap(err, "error looking up nsenter path")
}

func ensureDaemonNetworks() error {
	var err error
	daemonNetworksOnce.Do(func() {
		daemonNetworks, err = cni.New(cni.WithLoNetwork, cni.WithDefaultConf)
		err = errors.Wrap(err, "error setting up cni for daemon networks")
	})

	return err
}

// Exec generates a command which, when executed, will run in the daemon's context
// In the case of Linux, this means it will run in the daemon's network namespace
func (d *Daemon) Exec(command string, args ...string) icmd.Cmd {
	return icmd.Command("nsenter", append([]string{"--net=" + d.netnsPath(), command}, args...)...)
}
