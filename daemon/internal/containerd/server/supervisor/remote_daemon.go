package supervisor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/cmd/containerd/server/config"
	"github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/containerd/v2/pkg/dialer"
	"github.com/containerd/log"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/moby/v2/pkg/pidfile"
	"github.com/moby/moby/v2/pkg/process"
	"github.com/pelletier/go-toml/v2"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	maxConnectionRetryCount = 3
	healthCheckTimeout      = 3 * time.Second
	shutdownTimeout         = 15 * time.Second
	startupTimeout          = 15 * time.Second
	configFile              = "containerd.toml"
	pidFile                 = "containerd.pid"
)

// Daemon configures, starts, and monitors a containerd daemon.
type Daemon struct {
	config config.Config

	// configFile is the location where the generated containerd configuration
	// file is saved.
	configFile string
	stateDir   string

	// daemonPath is the binary to execute, and can be either a basename (to use
	// a binary installed in the system's $PATH), or the full path to the binary
	// to use.
	daemonPath string
	daemonPid  int
	pidFile    string
	logger     *log.Entry

	daemonWaitCh  chan struct{}
	daemonStartCh chan error
	daemonStopCh  chan struct{}
}

// DaemonOpt allows to configure parameters of container daemons
type DaemonOpt func(c *Daemon) error

// New creates a containerd daemon supervisor.
//
// It uses rootDir for persistent state and the daemon subdirectory under
// stateDir for runtime state.
func New(rootDir, stateDir string, opts ...DaemonOpt) (*Daemon, error) {
	r := &Daemon{
		config: config.Config{
			Version: 2, // FIXME(thaJeztah): update to v3 when we drop support for containerd v1.
			Root:    rootDir,
			State:   filepath.Join(stateDir, "daemon"),
			GRPC: config.GRPCConfig{ //nolint:staticcheck // Deprecated in config v4, but required for config v3.
				Address:        defaultGRPCAddress(stateDir),   //nolint:staticcheck // Deprecated in config v4, but required for config v3.
				MaxRecvMsgSize: defaults.DefaultMaxRecvMsgSize, //nolint:staticcheck // Deprecated in config v4, but required for config v3.
				MaxSendMsgSize: defaults.DefaultMaxSendMsgSize, //nolint:staticcheck // Deprecated in config v4, but required for config v3.
			},
			Debug: config.Debug{
				Address: defaultDebugAddress(stateDir), //nolint:staticcheck // Deprecated in config v4, but required for config v3.
			},
		},
		stateDir:      stateDir,
		configFile:    filepath.Join(stateDir, configFile),
		daemonPath:    binaryName,
		daemonPid:     -1,
		pidFile:       filepath.Join(stateDir, pidFile),
		daemonStartCh: make(chan error, 1),
		daemonStopCh:  make(chan struct{}),
	}

	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}

	path, err := exec.LookPath(r.daemonPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find containerd binary: %s", r.daemonPath)
	}
	r.daemonPath = path

	return r, nil
}

// Start starts the containerd daemon and monitors it until ctx is canceled.
func (r *Daemon) Start(ctx context.Context) error {
	path, err := exec.LookPath(r.daemonPath)
	if err != nil {
		return errors.Wrapf(err, "failed to find containerd binary: %s", r.daemonPath)
	}
	r.daemonPath = path
	r.logger = log.G(ctx).WithFields(log.Fields{
		"binary": r.daemonPath,
		"module": "supervisor",
	})

	if err := os.MkdirAll(r.stateDir, 0o700); err != nil {
		return err
	}

	go r.monitorDaemon(ctx)

	timeout := time.NewTimer(startupTimeout)
	defer timeout.Stop()

	select {
	case <-timeout.C:
		return errors.New("timeout waiting for containerd to start")
	case err := <-r.daemonStartCh:
		if err != nil {
			return err
		}
		r.logger.Info("started managed containerd")
		return nil
	}
}

func (r *Daemon) WaitTimeout(d time.Duration) error {
	timeout := time.NewTimer(d)
	defer timeout.Stop()

	select {
	case <-timeout.C:
		return errors.New("timeout waiting for containerd to stop")
	case <-r.daemonStopCh:
	}

	return nil
}

func (r *Daemon) Address() string {
	return r.config.GRPC.Address //nolint:staticcheck // Deprecated in config v4, but required for config v3.
}

func (r *Daemon) getContainerdConfig() (string, error) {
	f, err := os.OpenFile(r.configFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open containerd config file (%s)", r.configFile)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(r.config); err != nil {
		return "", errors.Wrapf(err, "failed to write containerd config file (%s)", r.configFile)
	}
	return r.configFile, nil
}

func (r *Daemon) startContainerd() error {
	pid, err := pidfile.Read(r.pidFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if pid > 0 {
		r.daemonPid = pid
		r.logger = r.logger.WithField("pid", pid)
		r.logger.Info("containerd is still running")
		return nil
	}

	cfgFile, err := r.getContainerdConfig()
	if err != nil {
		return err
	}

	r.logger.Debug("starting containerd binary")
	// Docker clears its umask on startup.
	// Managed containerd inherits that umask, so paths it creates use the
	// literal modes requested by containerd instead of being masked by the
	// historical 0o022 process umask.
	//
	// This can make internal containerd paths more permissive than older
	// installs.
	// Under the default layout those paths remain below daemon-managed root
	// and state directories that deny access to "other", so they are reachable
	// only by root and the docker group, which is already equivalent to root.
	cmd := exec.Command(r.daemonPath, "--config", cfgFile)
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
		if err != nil {
			startedCh <- err
			return
		}
		r.daemonWaitCh = make(chan struct{})
		startedCh <- nil

		// Reap our child when needed
		if err := cmd.Wait(); err != nil {
			r.logger.WithError(err).Errorf("containerd did not exit successfully")
		}
		close(r.daemonWaitCh)
	}()
	if err := <-startedCh; err != nil {
		return err
	}

	r.daemonPid = cmd.Process.Pid

	if err := pidfile.Write(r.pidFile, r.daemonPid); err != nil {
		_ = process.Kill(r.daemonPid)
		return errors.Wrap(err, "failed to save containerd pid to disk")
	}

	r.logger = r.logger.WithField("pid", r.daemonPid)
	r.logger.WithField("address", r.Address()).Info("started new containerd process")

	return nil
}

func (r *Daemon) monitorDaemon(ctx context.Context) {
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
			r.logger.Info("stopping containerd healthcheck following graceful shutdown")
			if client != nil {
				_ = client.Close()
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

			grpcAddress := r.Address()
			if err := os.RemoveAll(grpcAddress); err != nil {
				r.logger.WithError(err).Error("failed to remove old gRPC address")
			}
			if err := r.startContainerd(); err != nil {
				if !started {
					r.daemonStartCh <- err
					return
				}
				r.logger.WithError(err).Error("failed restarting containerd")
				delay = 50 * time.Millisecond
				continue
			}

			client, err = containerd.New(
				grpcAddress,
				containerd.WithTimeout(60*time.Second),
				containerd.WithDialOpts([]grpc.DialOption{
					grpc.WithTransportCredentials(insecure.NewCredentials()),
					grpc.WithContextDialer(dialer.ContextDialer),
					grpc.WithUnaryInterceptor(grpcerrors.UnaryClientInterceptor),
					grpc.WithStreamInterceptor(grpcerrors.StreamClientInterceptor),
				}),
			)
			if err != nil {
				r.logger.WithError(err).Error("failed connecting to containerd")
				delay = 100 * time.Millisecond
				continue
			}
			r.logger.WithField("address", grpcAddress).Debug("created containerd monitoring client")
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

			r.logger.WithFields(log.Fields{
				"error":   err,
				"pid":     r.daemonPid,
				"retries": transientFailureCount,
			}).Debug("daemon is not responding")

			transientFailureCount++
			if transientFailureCount < maxConnectionRetryCount || process.Alive(r.daemonPid) {
				delay = time.Duration(transientFailureCount) * 200 * time.Millisecond
				continue
			}
			_ = client.Close()
			client = nil
		}

		if process.Alive(r.daemonPid) {
			r.logger.WithField("pid", r.daemonPid).Info("killing and restarting containerd")
			r.killDaemon()
		}

		r.daemonPid = -1
		delay = 0
		transientFailureCount = 0
	}
}
