package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/services/server/config"
	"github.com/containerd/containerd/sys"
	"github.com/containerd/log"
	"github.com/docker/docker/pkg/pidfile"
	"github.com/docker/docker/pkg/process"
	"github.com/docker/docker/pkg/system"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	maxConnectionRetryCount = 3
	healthCheckTimeout      = 3 * time.Second
	shutdownTimeout         = 15 * time.Second
	startupTimeout          = 15 * time.Second
	configFile              = "containerd.toml"
	binaryName              = "containerd"
	pidFile                 = "containerd.pid"
)

type remote struct {
	config.Config

	// configFile is the location where the generated containerd configuration
	// file is saved.
	configFile string

	pidFile string
	logger  *log.Entry

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
		pidFile:       filepath.Join(stateDir, pidFile),
		logger:        log.G(ctx).WithField("module", "libcontainerd"),
		daemonStartCh: make(chan error, 1),
		daemonStopCh:  make(chan struct{}),
	}

	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	r.setDefaults()

	if err := system.MkdirAll(stateDir, 0o700); err != nil {
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

func (r *remote) getContainerdConfig() (string, error) {
	f, err := os.OpenFile(r.configFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open containerd config file (%s)", r.configFile)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(r); err != nil {
		return "", errors.Wrapf(err, "failed to write containerd config file (%s)", r.configFile)
	}
	return r.configFile, nil
}

func (r *remote) startContainerd() (pid int, daemonWaitCh <-chan struct{}, err error) {
	pid, err = pidfile.Read(r.pidFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return -1, nil, err
	}

	if pid > 0 {
		r.logger.WithField("pid", pid).Infof("%s is still running", binaryName)
		return pid, nil, nil
	}

	cfgFile, err := r.getContainerdConfig()
	if err != nil {
		return -1, nil, err
	}
	args := []string{"--config", cfgFile}

	if r.logLevel != "" {
		args = append(args, "--log-level", r.logLevel)
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

	startedCh := make(chan error)
	waitCh := make(chan struct{})
	go func() {
		// On Linux, when cmd.SysProcAttr.Pdeathsig is set,
		// the signal is sent to the subprocess when the creating thread
		// terminates. The runtime terminates a thread if a goroutine
		// exits while locked to it. Prevent the containerd process
		// from getting killed prematurely by ensuring that the thread
		// used to start it remains alive until it or the daemon process
		// exits. See https://go.dev/issue/27505 for more details.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		err := cmd.Start()
		startedCh <- err
		if err != nil {
			return
		}

		// Reap our child when needed
		if err := cmd.Wait(); err != nil {
			r.logger.WithError(err).Errorf("containerd did not exit successfully")
		}
		close(waitCh)
	}()
	if err := <-startedCh; err != nil {
		return -1, nil, err
	}

	pid = cmd.Process.Pid

	if r.oomScore != 0 {
		if err := sys.SetOOMScore(pid, r.oomScore); err != nil {
			r.logger.WithField("pid", pid).Warn("failed to adjust OOM score for containerd process")
		}
	}

	if err := pidfile.Write(r.pidFile, pid); err != nil {
		_ = process.Kill(pid)
		return pid, waitCh, errors.Wrap(err, "libcontainerd: failed to save daemon pid to disk")
	}

	r.logger.WithField("pid", pid).WithField("address", r.Address()).Infof("started new %s process", binaryName)

	return pid, waitCh, nil
}

func (r *remote) monitorDaemon(ctx context.Context) {
	var (
		transientFailureCount = 0
		daemonPid             = -1
		daemonWaitCh          <-chan struct{}
		client                *containerd.Client
		err                   error
		delay                 time.Duration
		timer                 = time.NewTimer(0)
		started               bool
	)

	defer func() {
		if daemonPid != -1 {
			r.stopDaemon(daemonPid)
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

		if daemonPid == -1 {
			if daemonWaitCh != nil {
				select {
				case <-ctx.Done():
					r.logger.Info("stopping containerd startup following graceful shutdown")
					return
				case <-daemonWaitCh:
				}
			}

			os.RemoveAll(r.GRPC.Address)
			if daemonPid, daemonWaitCh, err = r.startContainerd(); err != nil {
				if !started {
					r.daemonStartCh <- err
					return
				}
				r.logger.WithError(err).Error("failed restarting containerd")
				delay = 50 * time.Millisecond
				continue
			}

			client, err = containerd.New(
				r.GRPC.Address,
				containerd.WithTimeout(60*time.Second),
				containerd.WithDialOpts([]grpc.DialOption{
					grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
					grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
					grpc.WithTransportCredentials(insecure.NewCredentials()),
					grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(defaults.DefaultMaxRecvMsgSize)),
					grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(defaults.DefaultMaxSendMsgSize)),
				}),
			)
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
				case <-daemonWaitCh:
				case <-ctx.Done():
				}

				// Set a small delay in case there is a recurring failure (or bug in this code)
				// to ensure we don't end up in a super tight loop.
				delay = 500 * time.Millisecond
				continue
			}

			r.logger.WithError(err).WithField("binary", binaryName).Debug("daemon is not responding")

			transientFailureCount++
			if transientFailureCount < maxConnectionRetryCount || process.Alive(daemonPid) {
				delay = time.Duration(transientFailureCount) * 200 * time.Millisecond
				continue
			}
			client.Close()
			client = nil
		}

		if process.Alive(daemonPid) {
			r.logger.WithField("pid", daemonPid).Info("killing and restarting containerd")
			r.killDaemon(daemonPid)
		}

		daemonPid = -1
		delay = 0
		transientFailureCount = 0
	}
}
