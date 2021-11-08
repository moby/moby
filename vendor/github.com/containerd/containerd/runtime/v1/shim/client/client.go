//go:build !windows
// +build !windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/log"
	v1 "github.com/containerd/containerd/runtime/v1"
	"github.com/containerd/containerd/runtime/v1/shim"
	shimapi "github.com/containerd/containerd/runtime/v1/shim/v1"
	"github.com/containerd/containerd/sys"
	"github.com/containerd/ttrpc"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	exec "golang.org/x/sys/execabs"
	"golang.org/x/sys/unix"
)

var empty = &ptypes.Empty{}

// Opt is an option for a shim client configuration
type Opt func(context.Context, shim.Config) (shimapi.ShimService, io.Closer, error)

// WithStart executes a new shim process
func WithStart(binary, address, daemonAddress, cgroup string, debug bool, exitHandler func()) Opt {
	return func(ctx context.Context, config shim.Config) (_ shimapi.ShimService, _ io.Closer, err error) {
		socket, err := newSocket(address)
		if err != nil {
			if !eaddrinuse(err) {
				return nil, nil, err
			}
			if err := RemoveSocket(address); err != nil {
				return nil, nil, errors.Wrap(err, "remove already used socket")
			}
			if socket, err = newSocket(address); err != nil {
				return nil, nil, err
			}
		}

		f, err := socket.File()
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to get fd for socket %s", address)
		}
		defer f.Close()

		stdoutCopy := io.Discard
		stderrCopy := io.Discard
		stdoutLog, err := v1.OpenShimStdoutLog(ctx, config.WorkDir)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to create stdout log")
		}

		stderrLog, err := v1.OpenShimStderrLog(ctx, config.WorkDir)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to create stderr log")
		}
		if debug {
			stdoutCopy = os.Stdout
			stderrCopy = os.Stderr
		}

		go io.Copy(stdoutCopy, stdoutLog)
		go io.Copy(stderrCopy, stderrLog)

		cmd, err := newCommand(binary, daemonAddress, debug, config, f, stdoutLog, stderrLog)
		if err != nil {
			return nil, nil, err
		}
		if err := cmd.Start(); err != nil {
			return nil, nil, errors.Wrapf(err, "failed to start shim")
		}
		defer func() {
			if err != nil {
				cmd.Process.Kill()
			}
		}()
		go func() {
			cmd.Wait()
			exitHandler()
			if stdoutLog != nil {
				stdoutLog.Close()
			}
			if stderrLog != nil {
				stderrLog.Close()
			}
			socket.Close()
			RemoveSocket(address)
		}()
		log.G(ctx).WithFields(logrus.Fields{
			"pid":     cmd.Process.Pid,
			"address": address,
			"debug":   debug,
		}).Infof("shim %s started", binary)

		if err := writeFile(filepath.Join(config.Path, "address"), address); err != nil {
			return nil, nil, err
		}
		if err := writeFile(filepath.Join(config.Path, "shim.pid"), strconv.Itoa(cmd.Process.Pid)); err != nil {
			return nil, nil, err
		}
		// set shim in cgroup if it is provided
		if cgroup != "" {
			if err := setCgroup(cgroup, cmd); err != nil {
				return nil, nil, err
			}
			log.G(ctx).WithFields(logrus.Fields{
				"pid":     cmd.Process.Pid,
				"address": address,
			}).Infof("shim placed in cgroup %s", cgroup)
		}
		if err = setupOOMScore(cmd.Process.Pid); err != nil {
			return nil, nil, err
		}
		c, clo, err := WithConnect(address, func() {})(ctx, config)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to connect")
		}
		return c, clo, nil
	}
}

func eaddrinuse(err error) bool {
	cause := errors.Cause(err)
	netErr, ok := cause.(*net.OpError)
	if !ok {
		return false
	}
	if netErr.Op != "listen" {
		return false
	}
	syscallErr, ok := netErr.Err.(*os.SyscallError)
	if !ok {
		return false
	}
	errno, ok := syscallErr.Err.(syscall.Errno)
	if !ok {
		return false
	}
	return errno == syscall.EADDRINUSE
}

// setupOOMScore gets containerd's oom score and adds +1 to it
// to ensure a shim has a lower* score than the daemons
// if not already at the maximum OOM Score
func setupOOMScore(shimPid int) error {
	pid := os.Getpid()
	score, err := sys.GetOOMScoreAdj(pid)
	if err != nil {
		return errors.Wrap(err, "get daemon OOM score")
	}
	shimScore := score + 1
	if err := sys.AdjustOOMScore(shimPid, shimScore); err != nil {
		return errors.Wrap(err, "set shim OOM score")
	}
	return nil
}

func newCommand(binary, daemonAddress string, debug bool, config shim.Config, socket *os.File, stdout, stderr io.Writer) (*exec.Cmd, error) {
	selfExe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	args := []string{
		"-namespace", config.Namespace,
		"-workdir", config.WorkDir,
		"-address", daemonAddress,
		"-containerd-binary", selfExe,
	}

	if config.Criu != "" {
		args = append(args, "-criu-path", config.Criu)
	}
	if config.RuntimeRoot != "" {
		args = append(args, "-runtime-root", config.RuntimeRoot)
	}
	if config.SystemdCgroup {
		args = append(args, "-systemd-cgroup")
	}
	if debug {
		args = append(args, "-debug")
	}

	cmd := exec.Command(binary, args...)
	cmd.Dir = config.Path
	// make sure the shim can be re-parented to system init
	// and is cloned in a new mount namespace because the overlay/filesystems
	// will be mounted by the shim
	cmd.SysProcAttr = getSysProcAttr()
	cmd.ExtraFiles = append(cmd.ExtraFiles, socket)
	cmd.Env = append(os.Environ(), "GOMAXPROCS=2")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd, nil
}

// writeFile writes a address file atomically
func writeFile(path, address string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	tempPath := filepath.Join(filepath.Dir(path), fmt.Sprintf(".%s", filepath.Base(path)))
	f, err := os.OpenFile(tempPath, os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_SYNC, 0666)
	if err != nil {
		return err
	}
	_, err = f.WriteString(address)
	f.Close()
	if err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

const (
	abstractSocketPrefix = "\x00"
	socketPathLimit      = 106
)

type socket string

func (s socket) isAbstract() bool {
	return !strings.HasPrefix(string(s), "unix://")
}

func (s socket) path() string {
	path := strings.TrimPrefix(string(s), "unix://")
	// if there was no trim performed, we assume an abstract socket
	if len(path) == len(s) {
		path = abstractSocketPrefix + path
	}
	return path
}

func newSocket(address string) (*net.UnixListener, error) {
	if len(address) > socketPathLimit {
		return nil, errors.Errorf("%q: unix socket path too long (> %d)", address, socketPathLimit)
	}
	var (
		sock = socket(address)
		path = sock.path()
	)
	if !sock.isAbstract() {
		if err := os.MkdirAll(filepath.Dir(path), 0600); err != nil {
			return nil, errors.Wrapf(err, "%s", path)
		}
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to listen to unix socket %q (abstract: %t)", address, sock.isAbstract())
	}
	if err := os.Chmod(path, 0600); err != nil {
		l.Close()
		return nil, err
	}

	return l.(*net.UnixListener), nil
}

// RemoveSocket removes the socket at the specified address if
// it exists on the filesystem
func RemoveSocket(address string) error {
	sock := socket(address)
	if !sock.isAbstract() {
		return os.Remove(sock.path())
	}
	return nil
}

// AnonDialer returns a dialer for a socket
//
// NOTE: It is only used for testing.
func AnonDialer(address string, timeout time.Duration) (net.Conn, error) {
	return anonDialer(address, timeout)
}

func connect(address string, d func(string, time.Duration) (net.Conn, error)) (net.Conn, error) {
	return d(address, 100*time.Second)
}

func anonDialer(address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", socket(address).path(), timeout)
}

// WithConnect connects to an existing shim
func WithConnect(address string, onClose func()) Opt {
	return func(ctx context.Context, config shim.Config) (shimapi.ShimService, io.Closer, error) {
		conn, err := connect(address, anonDialer)
		if err != nil {
			return nil, nil, err
		}
		client := ttrpc.NewClient(conn, ttrpc.WithOnClose(onClose))
		return shimapi.NewShimClient(client), conn, nil
	}
}

// WithLocal uses an in process shim
func WithLocal(publisher events.Publisher) func(context.Context, shim.Config) (shimapi.ShimService, io.Closer, error) {
	return func(ctx context.Context, config shim.Config) (shimapi.ShimService, io.Closer, error) {
		service, err := shim.NewService(config, publisher)
		if err != nil {
			return nil, nil, err
		}
		return shim.NewLocal(service), nil, nil
	}
}

// New returns a new shim client
func New(ctx context.Context, config shim.Config, opt Opt) (*Client, error) {
	s, c, err := opt(ctx, config)
	if err != nil {
		return nil, err
	}
	return &Client{
		ShimService: s,
		c:           c,
		exitCh:      make(chan struct{}),
	}, nil
}

// Client is a shim client containing the connection to a shim
type Client struct {
	shimapi.ShimService

	c        io.Closer
	exitCh   chan struct{}
	exitOnce sync.Once
}

// IsAlive returns true if the shim can be contacted.
// NOTE: a negative answer doesn't mean that the process is gone.
func (c *Client) IsAlive(ctx context.Context) (bool, error) {
	_, err := c.ShimInfo(ctx, empty)
	if err != nil {
		// TODO(stevvooe): There are some error conditions that need to be
		// handle with unix sockets existence to give the right answer here.
		return false, err
	}
	return true, nil
}

// StopShim signals the shim to exit and wait for the process to disappear
func (c *Client) StopShim(ctx context.Context) error {
	return c.signalShim(ctx, unix.SIGTERM)
}

// KillShim kills the shim forcefully and wait for the process to disappear
func (c *Client) KillShim(ctx context.Context) error {
	return c.signalShim(ctx, unix.SIGKILL)
}

// Close the client connection
func (c *Client) Close() error {
	if c.c == nil {
		return nil
	}
	return c.c.Close()
}

func (c *Client) signalShim(ctx context.Context, sig syscall.Signal) error {
	info, err := c.ShimInfo(ctx, empty)
	if err != nil {
		return err
	}
	pid := int(info.ShimPid)
	// make sure we don't kill ourselves if we are running a local shim
	if os.Getpid() == pid {
		return nil
	}
	if err := unix.Kill(pid, sig); err != nil && err != unix.ESRCH {
		return err
	}
	// wait for shim to die after being signaled
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.waitForExit(ctx, pid):
		return nil
	}
}

func (c *Client) waitForExit(ctx context.Context, pid int) <-chan struct{} {
	go c.exitOnce.Do(func() {
		defer close(c.exitCh)

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			// use kill(pid, 0) here because the shim could have been reparented
			// and we are no longer able to waitpid(pid, ...) on the shim
			if err := unix.Kill(pid, 0); err == unix.ESRCH {
				return
			}

			select {
			case <-ticker.C:
			case <-ctx.Done():
				log.G(ctx).WithField("pid", pid).Warn("timed out while waiting for shim to exit")
				return
			}
		}
	})
	return c.exitCh
}
