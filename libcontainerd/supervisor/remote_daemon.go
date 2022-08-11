package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/services/server/config"
	"github.com/containerd/containerd/sys"
	"github.com/docker/docker/pkg/system"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	maxConnectionRetryCount = 3
	healthCheckTimeout      = 3 * time.Second
	shutdownTimeout         = 15 * time.Second
	startupTimeout          = 15 * time.Second
	configFile              = "containerd.toml"
	pidFile                 = "containerd.pid"
)

type remote struct {
	config.Config

	// configFile is the location where the generated containerd configuration
	// file is saved.
	configFile string

	// daemonPath is the binary to execute, and can be either a basename (to use
	// a binary installed in the system's $PATH), or the full path to the binary
	// to use.
	daemonPath string
	daemonPid  int
	pidFile    string
	logger     *logrus.Entry

	daemonWaitCh  chan struct{}
	daemonStartCh chan error
	daemonStopCh  chan struct{}

	stateDir string

	// oomScore adjusts the OOM score for the containerd process.
	oomScore int

	// logLevel overrides the containerd logging-level through the --log-level
	// command-line option.
	logLevel string
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
		stateDir: stateDir,
		Config: config.Config{
			Version: 2,
			Root:    filepath.Join(rootDir, "daemon"),
			State:   filepath.Join(stateDir, "daemon"),
		},
		configFile:    filepath.Join(stateDir, configFile),
		daemonPath:    binaryName,
		daemonPid:     -1,
		pidFile:       filepath.Join(stateDir, pidFile),
		logger:        logrus.WithField("module", "libcontainerd"),
		daemonStartCh: make(chan error, 1),
		daemonStopCh:  make(chan struct{}),
	}

	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	r.setDefaults()

	if err := system.MkdirAll(stateDir, 0700); err != nil {
		return nil, err
	}

	go r.monitorDaemon(ctx)

	timeout := time.NewTimer(startupTimeout)
	defer timeout.Stop()

	select {
	case <-timeout.C:
		return nil, errors.New("timeout waiting for containerd to start")
	case err := <-r.daemonStartCh:
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}
func (r *remote) WaitTimeout(d time.Duration) error {
	timeout := time.NewTimer(d)
	defer timeout.Stop()

	select {
	case <-timeout.C:
		return errors.New("timeout waiting for containerd to stop")
	case <-r.daemonStopCh:
	}

	return nil
}

func (r *remote) Address() string {
	return r.GRPC.Address
}
func (r *remote) getContainerdPid() (int, error) {
	f, err := os.OpenFile(r.pidFile, os.O_RDWR, 0600)
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
	f, err := os.OpenFile(r.configFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open containerd config file (%s)", r.configFile)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(r); err != nil {
		return "", errors.Wrapf(err, "failed to write containerd config file (%s)", r.configFile)
	}
	return r.configFile, nil
}

func (r *remote) startContainerd() error {
	pid, err := r.getContainerdPid()
	if err != nil {
		return err
	}

	if pid != -1 {
		r.daemonPid = pid
		r.logger.WithField("pid", pid).Infof("%s is still running", binaryName)
		return nil
	}

	configFile, err := r.getContainerdConfig()
	if err != nil {
		return err
	}

	args := []string{"--config", configFile}

	if r.logLevel != "" {
		args = append(args, "--log-level", r.logLevel)
	}

	r.logger.WithField("binary", r.daemonPath).Debug("starting containerd binary")
	cmd := exec.Command(r.daemonPath, args...)
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

	if err := r.adjustOOMScore(); err != nil {
		r.logger.WithError(err).Warn("failed to adjust OOM score")
	}

	err = os.WriteFile(r.pidFile, []byte(strconv.Itoa(r.daemonPid)), 0660)
	if err != nil {
		system.KillProcess(r.daemonPid)
		return errors.Wrap(err, "libcontainerd: failed to save daemon pid to disk")
	}

	r.logger.WithField("pid", r.daemonPid).WithField("address", r.Address()).Infof("started new %s process", binaryName)

	return nil
}

func (r *remote) adjustOOMScore() error {
	if r.oomScore == 0 || r.daemonPid <= 1 {
		// no score configured, or daemonPid contains an invalid PID (we don't
		// expect containerd to be running as PID 1 :)).
		return nil
	}
	if err := sys.SetOOMScore(r.daemonPid, r.oomScore); err != nil {
		return errors.Wrap(err, "failed to adjust OOM score for containerd process")
	}
	return nil
}

func (r *remote) monitorDaemon(ctx context.Context) {
	var (
		transientFailureCount = 0
		client                *containerd.Client
		err                   error
		delay                 time.Duration
		timer                 = time.NewTimer(0)
		started               bool
	)

	defer func() {
		if r.daemonPid != -1 {
			r.stopDaemon()
		}

		// cleanup some files
		_ = os.Remove(r.pidFile)

		r.platformCleanup()

		close(r.daemonStopCh)
		timer.Stop()
	}()

	// ensure no races on sending to timer.C even though there is a 0 duration.
	if !timer.Stop() {
		<-timer.C
	}

	for {
		timer.Reset(delay)

		select {
		case <-ctx.Done():
			r.logger.Info("stopping healthcheck following graceful shutdown")
			if client != nil {
				client.Close()
			}
			return
		case <-timer.C:
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
				if !started {
					r.daemonStartCh <- err
					return
				}
				r.logger.WithError(err).Error("failed restarting containerd")
				delay = 50 * time.Millisecond
				continue
			}

			client, err = containerd.New(r.GRPC.Address, containerd.WithTimeout(60*time.Second))
			if err != nil {
				r.logger.WithError(err).Error("failed connecting to containerd")
				delay = 100 * time.Millisecond
				continue
			}
			r.logger.WithField("address", r.GRPC.Address).Debug("created containerd monitoring client")
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

				select {
				case <-r.daemonWaitCh:
				case <-ctx.Done():
				}

				// Set a small delay in case there is a recurring failure (or bug in this code)
				// to ensure we don't end up in a super tight loop.
				delay = 500 * time.Millisecond
				continue
			}

			r.logger.WithError(err).WithField("binary", binaryName).Debug("daemon is not responding")

			transientFailureCount++
			if transientFailureCount < maxConnectionRetryCount || system.IsProcessAlive(r.daemonPid) {
				delay = time.Duration(transientFailureCount) * 200 * time.Millisecond
				continue
			}
			client.Close()
			client = nil
		}

		if system.IsProcessAlive(r.daemonPid) {
			r.logger.WithField("pid", r.daemonPid).Info("killing and restarting containerd")
			r.killDaemon()
		}

		r.daemonPid = -1
		delay = 0
		transientFailureCount = 0
	}
}
