package libcontainerd

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	containerd "github.com/docker/containerd/api/grpc/types"
	"github.com/docker/docker/pkg/locker"
	sysinfo "github.com/docker/docker/pkg/system"
	"github.com/docker/docker/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/transport"
)

const (
	maxConnectionRetryCount   = 3
	connectionRetryDelay      = 3 * time.Second
	containerdShutdownTimeout = 15 * time.Second
	containerdBinary          = "docker-containerd"
	containerdPidFilename     = "docker-containerd.pid"
	containerdSockFilename    = "docker-containerd.sock"
	containerdStateDir        = "containerd"
	eventTimestampFilename    = "event.ts"
)

type remote struct {
	sync.RWMutex
	apiClient     containerd.APIClient
	daemonPid     int
	stateDir      string
	rpcAddr       string
	startDaemon   bool
	closeManually bool
	debugLog      bool
	rpcConn       *grpc.ClientConn
	clients       []*client
	eventTsPath   string
	pastEvents    map[string]*containerd.Event
	runtimeArgs   []string
	daemonWaitCh  chan struct{}
}

// New creates a fresh instance of libcontainerd remote.
func New(stateDir string, options ...RemoteOption) (_ Remote, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("Failed to connect to containerd. Please make sure containerd is installed in your PATH or you have specificed the correct address. Got error: %v", err)
		}
	}()
	r := &remote{
		stateDir:    stateDir,
		daemonPid:   -1,
		eventTsPath: filepath.Join(stateDir, eventTimestampFilename),
		pastEvents:  make(map[string]*containerd.Event),
	}
	for _, option := range options {
		if err := option.Apply(r); err != nil {
			return nil, err
		}
	}

	if err := sysinfo.MkdirAll(stateDir, 0700); err != nil {
		return nil, err
	}

	if r.rpcAddr == "" {
		r.rpcAddr = filepath.Join(stateDir, containerdSockFilename)
	}

	if r.startDaemon {
		if err := r.runContainerdDaemon(); err != nil {
			return nil, err
		}
	}

	// don't output the grpc reconnect logging
	grpclog.SetLogger(log.New(ioutil.Discard, "", log.LstdFlags))
	dialOpts := append([]grpc.DialOption{grpc.WithInsecure()},
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)
	conn, err := grpc.Dial(r.rpcAddr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("error connecting to containerd: %v", err)
	}

	r.rpcConn = conn
	r.apiClient = containerd.NewAPIClient(conn)

	go r.handleConnectionChange()

	if err := r.startEventsMonitor(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *remote) handleConnectionChange() {
	var transientFailureCount = 0
	state := grpc.Idle
	for {
		s, err := r.rpcConn.WaitForStateChange(context.Background(), state)
		if err != nil {
			break
		}
		state = s
		logrus.Debugf("containerd connection state change: %v", s)

		if r.daemonPid != -1 {
			switch state {
			case grpc.TransientFailure:
				// Reset state to be notified of next failure
				transientFailureCount++
				if transientFailureCount >= maxConnectionRetryCount {
					transientFailureCount = 0
					if utils.IsProcessAlive(r.daemonPid) {
						utils.KillProcess(r.daemonPid)
						<-r.daemonWaitCh
					}
					if err := r.runContainerdDaemon(); err != nil { //FIXME: Handle error
						logrus.Errorf("error restarting containerd: %v", err)
					}
				} else {
					state = grpc.Idle
					time.Sleep(connectionRetryDelay)
				}
			case grpc.Shutdown:
				// Well, we asked for it to stop, just return
				return
			}
		}
	}
}

func (r *remote) Cleanup() {
	if r.daemonPid == -1 {
		return
	}
	r.closeManually = true
	r.rpcConn.Close()
	// Ask the daemon to quit
	syscall.Kill(r.daemonPid, syscall.SIGTERM)

	// Wait up to 15secs for it to stop
	for i := time.Duration(0); i < containerdShutdownTimeout; i += time.Second {
		if !utils.IsProcessAlive(r.daemonPid) {
			break
		}
		time.Sleep(time.Second)
	}

	if utils.IsProcessAlive(r.daemonPid) {
		logrus.Warnf("libcontainerd: containerd (%d) didn't stop within 15 secs, killing it\n", r.daemonPid)
		syscall.Kill(r.daemonPid, syscall.SIGKILL)
	}

	// cleanup some files
	os.Remove(filepath.Join(r.stateDir, containerdPidFilename))
	os.Remove(filepath.Join(r.stateDir, containerdSockFilename))
}

func (r *remote) Client(b Backend) (Client, error) {
	c := &client{
		clientCommon: clientCommon{
			backend:    b,
			containers: make(map[string]*container),
			locker:     locker.New(),
		},
		remote:        r,
		exitNotifiers: make(map[string]*exitNotifier),
	}

	r.Lock()
	r.clients = append(r.clients, c)
	r.Unlock()
	return c, nil
}

func (r *remote) updateEventTimestamp(t time.Time) {
	f, err := os.OpenFile(r.eventTsPath, syscall.O_CREAT|syscall.O_WRONLY|syscall.O_TRUNC, 0600)
	defer f.Close()
	if err != nil {
		logrus.Warnf("libcontainerd: failed to open event timestamp file: %v", err)
		return
	}

	b, err := t.MarshalText()
	if err != nil {
		logrus.Warnf("libcontainerd: failed to encode timestamp: %v", err)
		return
	}

	n, err := f.Write(b)
	if err != nil || n != len(b) {
		logrus.Warnf("libcontainerd: failed to update event timestamp file: %v", err)
		f.Truncate(0)
		return
	}

}

func (r *remote) getLastEventTimestamp() int64 {
	t := time.Now()

	fi, err := os.Stat(r.eventTsPath)
	if os.IsNotExist(err) || fi.Size() == 0 {
		return t.Unix()
	}

	f, err := os.Open(r.eventTsPath)
	defer f.Close()
	if err != nil {
		logrus.Warn("libcontainerd: Unable to access last event ts: %v", err)
		return t.Unix()
	}

	b := make([]byte, fi.Size())
	n, err := f.Read(b)
	if err != nil || n != len(b) {
		logrus.Warn("libcontainerd: Unable to read last event ts: %v", err)
		return t.Unix()
	}

	t.UnmarshalText(b)

	return t.Unix()
}

func (r *remote) startEventsMonitor() error {
	// First, get past events
	er := &containerd.EventsRequest{
		Timestamp: uint64(r.getLastEventTimestamp()),
	}
	events, err := r.apiClient.Events(context.Background(), er)
	if err != nil {
		return err
	}
	go r.handleEventStream(events)
	return nil
}

func (r *remote) handleEventStream(events containerd.API_EventsClient) {
	live := false
	for {
		e, err := events.Recv()
		if err != nil {
			if grpc.ErrorDesc(err) == transport.ErrConnClosing.Desc &&
				r.closeManually {
				// ignore error if grpc remote connection is closed manually
				return
			}
			logrus.Errorf("failed to receive event from containerd: %v", err)
			go r.startEventsMonitor()
			return
		}

		if live == false {
			logrus.Debugf("received past containerd event: %#v", e)

			// Pause/Resume events should never happens after exit one
			switch e.Type {
			case StateExit:
				r.pastEvents[e.Id] = e
			case StatePause:
				r.pastEvents[e.Id] = e
			case StateResume:
				r.pastEvents[e.Id] = e
			case stateLive:
				live = true
				r.updateEventTimestamp(time.Unix(int64(e.Timestamp), 0))
			}
		} else {
			logrus.Debugf("received containerd event: %#v", e)

			var container *container
			var c *client
			r.RLock()
			for _, c = range r.clients {
				container, err = c.getContainer(e.Id)
				if err == nil {
					break
				}
			}
			r.RUnlock()
			if container == nil {
				logrus.Errorf("no state for container: %q", err)
				continue
			}

			if err := container.handleEvent(e); err != nil {
				logrus.Errorf("error processing state change for %s: %v", e.Id, err)
			}

			r.updateEventTimestamp(time.Unix(int64(e.Timestamp), 0))
		}
	}
}

func (r *remote) runContainerdDaemon() error {
	pidFilename := filepath.Join(r.stateDir, containerdPidFilename)
	f, err := os.OpenFile(pidFilename, os.O_RDWR|os.O_CREATE, 0600)
	defer f.Close()
	if err != nil {
		return err
	}

	// File exist, check if the daemon is alive
	b := make([]byte, 8)
	n, err := f.Read(b)
	if err != nil && err != io.EOF {
		return err
	}

	if n > 0 {
		pid, err := strconv.ParseUint(string(b[:n]), 10, 64)
		if err != nil {
			return err
		}
		if utils.IsProcessAlive(int(pid)) {
			logrus.Infof("previous instance of containerd still alive (%d)", pid)
			r.daemonPid = int(pid)
			return nil
		}
	}

	// rewind the file
	_, err = f.Seek(0, os.SEEK_SET)
	if err != nil {
		return err
	}

	// Truncate it
	err = f.Truncate(0)
	if err != nil {
		return err
	}

	// Start a new instance
	args := []string{
		"-l", fmt.Sprintf("unix://%s", r.rpcAddr),
		"--shim", "docker-containerd-shim",
		"--runtime", "docker-runc",
		"--metrics-interval=0",
		"--state-dir", filepath.Join(r.stateDir, containerdStateDir),
	}
	if r.debugLog {
		args = append(args, "--debug")
	}
	if len(r.runtimeArgs) > 0 {
		for _, v := range r.runtimeArgs {
			args = append(args, "--runtime-args")
			args = append(args, v)
		}
		logrus.Debugf("runContainerdDaemon: runtimeArgs: %s", args)
	}

	cmd := exec.Command(containerdBinary, args...)
	// redirect containerd logs to docker logs
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Env = nil
	// clear the NOTIFY_SOCKET from the env when starting containerd
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "NOTIFY_SOCKET") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	logrus.Infof("New containerd process, pid: %d", cmd.Process.Pid)

	if _, err := f.WriteString(fmt.Sprintf("%d", cmd.Process.Pid)); err != nil {
		utils.KillProcess(cmd.Process.Pid)
		return err
	}

	r.daemonWaitCh = make(chan struct{})
	go func() {
		cmd.Wait()
		close(r.daemonWaitCh)
	}() // Reap our child when needed
	r.daemonPid = cmd.Process.Pid
	return nil
}

// WithRemoteAddr sets the external containerd socket to connect to.
func WithRemoteAddr(addr string) RemoteOption {
	return rpcAddr(addr)
}

type rpcAddr string

func (a rpcAddr) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.rpcAddr = string(a)
		return nil
	}
	return fmt.Errorf("WithRemoteAddr option not supported for this remote")
}

// WithRuntimeArgs sets the list of runtime args passed to containerd
func WithRuntimeArgs(args []string) RemoteOption {
	return runtimeArgs(args)
}

type runtimeArgs []string

func (rt runtimeArgs) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.runtimeArgs = rt
		return nil
	}
	return fmt.Errorf("WithRuntimeArgs option not supported for this remote")
}

// WithStartDaemon defines if libcontainerd should also run containerd daemon.
func WithStartDaemon(start bool) RemoteOption {
	return startDaemon(start)
}

type startDaemon bool

func (s startDaemon) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.startDaemon = bool(s)
		return nil
	}
	return fmt.Errorf("WithStartDaemon option not supported for this remote")
}

// WithDebugLog defines if containerd debug logs will be enabled for daemon.
func WithDebugLog(debug bool) RemoteOption {
	return debugLog(debug)
}

type debugLog bool

func (d debugLog) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.debugLog = bool(d)
		return nil
	}
	return fmt.Errorf("WithDebugLog option not supported for this remote")
}
