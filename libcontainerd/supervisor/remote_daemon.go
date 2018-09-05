package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/services/server"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	maxConnectionRetryCount = 3
	healthCheckTimeout      = 3 * time.Second
	shutdownTimeout         = 15 * time.Second
	startupTimeout          = 15 * time.Second
	configFile              = "containerd.toml"
	binaryName              = "docker-containerd"
	pidFile                 = "docker-containerd.pid"
)

type pluginConfigs struct {
	Plugins map[string]interface{} `toml:"plugins"`
}

type remote struct {
	sync.RWMutex
	server.Config

	daemonPid int
	logger    *logrus.Entry

	daemonWaitCh  chan struct{}
	daemonStartCh chan struct{}
	daemonStopCh  chan struct{}

	rootDir     string
	stateDir    string
	pluginConfs pluginConfigs
}

// Daemon represents a running containerd daemon
type Daemon interface {
	WaitTimeout(time.Duration) error
	Address() string
}

// DaemonOpt allows to configure parameters of container daemons
type DaemonOpt func(c *remote) error

// Start starts a containerd daemon and monitors it
func Start(ctx context.Context, rootDir, stateDir string, opts ...DaemonOpt) (Daemon, error) {
	r := &remote{
		rootDir:  rootDir,
		stateDir: stateDir,
		Config: server.Config{
			Root:  filepath.Join(rootDir, "daemon"),
			State: filepath.Join(stateDir, "daemon"),
		},
		pluginConfs:   pluginConfigs{make(map[string]interface{})},
		daemonPid:     -1,
		logger:        logrus.WithField("module", "libcontainerd"),
		daemonStartCh: make(chan struct{}),
		daemonStopCh:  make(chan struct{}),
	}

	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	r.setDefaults()

	if err := system.MkdirAll(stateDir, 0700, ""); err != nil {
		return nil, err
	}

	go r.monitorDaemon(ctx)

	select {
	case <-time.After(startupTimeout):
		return nil, errors.New("timeout waiting for containerd to start")
	case <-r.daemonStartCh:
	}

	return r, nil
}
func (r *remote) WaitTimeout(d time.Duration) error {
	select {
	case <-time.After(d):
		return errors.New("timeout waiting for containerd to stop")
	case <-r.daemonStopCh:
	}

	return nil
}

func (r *remote) Address() string {
	return r.GRPC.Address
}
func (r *remote) getContainerdPid() (int, error) {
	pidFile := filepath.Join(r.stateDir, pidFile)
	f, err := os.OpenFile(pidFile, os.O_RDWR, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			return -1, nil
		}
		return -1, err
	}
	defer f.Close()

	b := make([]byte, 8)
	n, err := f.Read(b)
	if err != nil && err != io.EOF {
		return -1, err
	}

	if n > 0 {
		pid, err := strconv.ParseUint(string(b[:n]), 10, 64)
		if err != nil {
			return -1, err
		}
		if system.IsProcessAlive(int(pid)) {
			return int(pid), nil
		}
	}

	return -1, nil
}

func (r *remote) getContainerdConfig() (string, error) {
	path := filepath.Join(r.stateDir, configFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open containerd config file at %s", path)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	if err = enc.Encode(r.Config); err != nil {
		return "", errors.Wrapf(err, "failed to encode general config")
	}
	if err = enc.Encode(r.pluginConfs); err != nil {
		return "", errors.Wrapf(err, "failed to encode plugin configs")
	}

	return path, nil
}

func (r *remote) startContainerd() error {
	pid, err := r.getContainerdPid()
	if err != nil {
		return err
	}

	if pid != -1 {
		r.daemonPid = pid
		logrus.WithField("pid", pid).
			Infof("libcontainerd: %s is still running", binaryName)
		return nil
	}

	configFile, err := r.getContainerdConfig()
	if err != nil {
		return err
	}

	args := []string{"--config", configFile}

	if r.Debug.Level != "" {
		args = append(args, "--log-level", r.Debug.Level)
	}

	cmd := exec.Command(binaryName, args...)
	// redirect containerd logs to docker logs
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = containerdSysProcAttr()
	// clear the NOTIFY_SOCKET from the env when starting containerd
	cmd.Env = nil
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "NOTIFY_SOCKET") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	r.daemonWaitCh = make(chan struct{})
	go func() {
		// Reap our child when needed
		if err := cmd.Wait(); err != nil {
			r.logger.WithError(err).Errorf("containerd did not exit successfully")
		}
		close(r.daemonWaitCh)
	}()

	r.daemonPid = cmd.Process.Pid

	err = ioutil.WriteFile(filepath.Join(r.stateDir, pidFile), []byte(fmt.Sprintf("%d", r.daemonPid)), 0660)
	if err != nil {
		system.KillProcess(r.daemonPid)
		return errors.Wrap(err, "libcontainerd: failed to save daemon pid to disk")
	}

	logrus.WithField("pid", r.daemonPid).
		Infof("libcontainerd: started new %s process", binaryName)

	return nil
}

func (r *remote) monitorDaemon(ctx context.Context) {
	var (
		transientFailureCount = 0
		client                *containerd.Client
		err                   error
		delay                 <-chan time.Time
		started               bool
	)

	defer func() {
		if r.daemonPid != -1 {
			r.stopDaemon()
		}

		// cleanup some files
		os.Remove(filepath.Join(r.stateDir, pidFile))

		r.platformCleanup()

		close(r.daemonStopCh)
	}()

	for {
		if delay != nil {
			select {
			case <-ctx.Done():
				r.logger.Info("stopping healthcheck following graceful shutdown")
				if client != nil {
					client.Close()
				}
				return
			case <-delay:
			}
		}

		if r.daemonPid == -1 {
			if r.daemonWaitCh != nil {
				select {
				case <-ctx.Done():
					r.logger.Info("stopping containerd startup following graceful shutdown")
					return
				case <-r.daemonWaitCh:
				}
			}

			os.RemoveAll(r.GRPC.Address)
			if err := r.startContainerd(); err != nil {
				r.logger.WithError(err).Error("failed starting containerd")
				delay = time.After(50 * time.Millisecond)
				continue
			}

			client, err = containerd.New(r.GRPC.Address)
			if err != nil {
				r.logger.WithError(err).Error("failed connecting to containerd")
				delay = time.After(100 * time.Millisecond)
				continue
			}
		}

		if client != nil {
			tctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
			_, err := client.IsServing(tctx)
			cancel()
			if err == nil {
				if !started {
					close(r.daemonStartCh)
					started = true
				}

				transientFailureCount = 0
				delay = time.After(500 * time.Millisecond)
				continue
			}

			r.logger.WithError(err).WithField("binary", binaryName).Debug("daemon is not responding")

			transientFailureCount++
			if transientFailureCount < maxConnectionRetryCount || system.IsProcessAlive(r.daemonPid) {
				delay = time.After(time.Duration(transientFailureCount) * 200 * time.Millisecond)
				continue
			}
		}

		if system.IsProcessAlive(r.daemonPid) {
			r.logger.WithField("pid", r.daemonPid).Info("killing and restarting containerd")
			r.killDaemon()
		}

		client.Close()
		client = nil
		r.daemonPid = -1
		delay = nil
		transientFailureCount = 0
	}
}
