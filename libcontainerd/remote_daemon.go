// +build !windows

package libcontainerd

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
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/server"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	maxConnectionRetryCount = 3
	healthCheckTimeout      = 3 * time.Second
	shutdownTimeout         = 15 * time.Second
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

	daemonWaitCh    chan struct{}
	clients         []*client
	shutdownContext context.Context
	shutdownCancel  context.CancelFunc
	shutdown        bool

	// Options
	startDaemon bool
	rootDir     string
	stateDir    string
	snapshotter string
	pluginConfs pluginConfigs
}

// New creates a fresh instance of libcontainerd remote.
func New(rootDir, stateDir string, options ...RemoteOption) (rem Remote, err error) {
	defer func() {
		if err != nil {
			err = errors.Wrap(err, "Failed to connect to containerd")
		}
	}()

	r := &remote{
		rootDir:  rootDir,
		stateDir: stateDir,
		Config: server.Config{
			Root:  filepath.Join(rootDir, "daemon"),
			State: filepath.Join(stateDir, "daemon"),
		},
		pluginConfs: pluginConfigs{make(map[string]interface{})},
		daemonPid:   -1,
		logger:      logrus.WithField("module", "libcontainerd"),
	}
	r.shutdownContext, r.shutdownCancel = context.WithCancel(context.Background())

	rem = r
	for _, option := range options {
		if err = option.Apply(r); err != nil {
			return
		}
	}
	r.setDefaults()

	if err = system.MkdirAll(stateDir, 0700, ""); err != nil {
		return
	}

	if r.startDaemon {
		os.Remove(r.GRPC.Address)
		if err = r.startContainerd(); err != nil {
			return
		}
		defer func() {
			if err != nil {
				r.Cleanup()
			}
		}()
	}

	// This connection is just used to monitor the connection
	client, err := containerd.New(r.GRPC.Address)
	if err != nil {
		return
	}
	if _, err := client.Version(context.Background()); err != nil {
		system.KillProcess(r.daemonPid)
		return nil, errors.Wrapf(err, "unable to get containerd version")
	}

	go r.monitorConnection(client)

	return r, nil
}

func (r *remote) NewClient(ns string, b Backend) (Client, error) {
	c := &client{
		stateDir:   r.stateDir,
		logger:     r.logger.WithField("namespace", ns),
		namespace:  ns,
		backend:    b,
		containers: make(map[string]*container),
	}

	rclient, err := containerd.New(r.GRPC.Address, containerd.WithDefaultNamespace(ns))
	if err != nil {
		return nil, err
	}
	c.remote = rclient

	go c.processEventStream(r.shutdownContext)

	r.Lock()
	r.clients = append(r.clients, c)
	r.Unlock()
	return c, nil
}

func (r *remote) Cleanup() {
	if r.daemonPid != -1 {
		r.shutdownCancel()
		r.stopDaemon()
	}

	// cleanup some files
	os.Remove(filepath.Join(r.stateDir, pidFile))

	r.platformCleanup()
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

func (r *remote) monitorConnection(client *containerd.Client) {
	var transientFailureCount = 0

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		<-ticker.C
		ctx, cancel := context.WithTimeout(r.shutdownContext, healthCheckTimeout)
		_, err := client.IsServing(ctx)
		cancel()
		if err == nil {
			transientFailureCount = 0
			continue
		}

		select {
		case <-r.shutdownContext.Done():
			r.logger.Info("stopping healtcheck following graceful shutdown")
			client.Close()
			return
		default:
		}

		r.logger.WithError(err).WithField("binary", binaryName).Debug("daemon is not responding")

		if r.daemonPid != -1 {
			transientFailureCount++
			if transientFailureCount >= maxConnectionRetryCount || !system.IsProcessAlive(r.daemonPid) {
				transientFailureCount = 0
				if system.IsProcessAlive(r.daemonPid) {
					r.logger.WithField("pid", r.daemonPid).Info("killing and restarting containerd")
					// Try to get a stack trace
					syscall.Kill(r.daemonPid, syscall.SIGUSR1)
					<-time.After(100 * time.Millisecond)
					system.KillProcess(r.daemonPid)
				}
				<-r.daemonWaitCh
				var err error
				client.Close()
				os.Remove(r.GRPC.Address)
				if err = r.startContainerd(); err != nil {
					r.logger.WithError(err).Error("failed restarting containerd")
				} else {
					newClient, err := containerd.New(r.GRPC.Address)
					if err != nil {
						r.logger.WithError(err).Error("failed connect to containerd")
					} else {
						client = newClient
					}
				}
			}
		}
	}
}
