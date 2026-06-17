//go:build !windows

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

package shim

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/log"
	"github.com/mdlayher/vsock"

	"github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/sys"
)

const (
	shimBinaryFormat = "containerd-shim-%s-%s"
	socketPathLimit  = 106
	protoVsock       = "vsock"
	protoHybridVsock = "hvsock"
	protoUnix        = "unix"
)

func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// AdjustOOMScore sets the OOM score for the process to the parents OOM score +1
// to ensure that they parent has a lower* score than the shim
// if not already at the maximum OOM Score
func AdjustOOMScore(pid int) error {
	parent := os.Getppid()
	score, err := sys.GetOOMScoreAdj(parent)
	if err != nil {
		return fmt.Errorf("get parent OOM score: %w", err)
	}
	shimScore := score + 1
	if err := sys.AdjustOOMScore(pid, shimScore); err != nil {
		return fmt.Errorf("set shim OOM score: %w", err)
	}
	return nil
}

const socketRoot = defaults.DefaultStateDir

// SocketAddress returns a socket address
func SocketAddress(ctx context.Context, socketPath, id string, debug bool) (string, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return "", err
	}
	path := filepath.Join(socketPath, ns, id)
	if debug {
		path = filepath.Join(path, "debug")
	}
	d := sha256.Sum256([]byte(path))
	return fmt.Sprintf("unix://%s/%x", filepath.Join(socketRoot, "s"), d), nil
}

// AnonDialer returns a dialer for a socket
func AnonDialer(address string, timeout time.Duration) (net.Conn, error) {
	proto, addr, ok := strings.Cut(address, "://")
	if !ok {
		return net.DialTimeout("unix", socket(address).path(), timeout)
	}
	switch proto {
	case protoVsock:
		// vsock dialer can not set timeout
		return dialVsock(addr)
	case protoHybridVsock:
		return dialHybridVsock(addr, timeout)
	case protoUnix:
		return net.DialTimeout("unix", socket(address).path(), timeout)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", proto)
	}
}

// AnonReconnectDialer returns a dialer for an existing socket on reconnection
func AnonReconnectDialer(address string, timeout time.Duration) (net.Conn, error) {
	return AnonDialer(address, timeout)
}

// NewSocket returns a new socket
func NewSocket(address string) (*net.UnixListener, error) {
	var (
		sock       = socket(address)
		path       = sock.path()
		isAbstract = sock.isAbstract()
		perm       = os.FileMode(0600)
	)

	// Darwin needs +x to access socket, otherwise it'll fail with "bind: permission denied" when running as non-root.
	if runtime.GOOS == "darwin" {
		perm = 0700
	}

	if !isAbstract {
		if err := os.MkdirAll(filepath.Dir(path), perm); err != nil {
			return nil, fmt.Errorf("mkdir failed for %s: %w", path, err)
		}
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	if !isAbstract {
		if err := os.Chmod(path, perm); err != nil {
			os.Remove(sock.path())
			l.Close()
			return nil, fmt.Errorf("chmod failed for %s: %w", path, err)
		}
	}

	return l.(*net.UnixListener), nil
}

const abstractSocketPrefix = "\x00"

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

// RemoveSocket removes the socket at the specified address if
// it exists on the filesystem
func RemoveSocket(address string) error {
	sock := socket(address)
	if !sock.isAbstract() {
		return os.Remove(sock.path())
	}
	return nil
}

// SocketEaddrinuse returns true if the provided error is caused by the
// EADDRINUSE error number
func SocketEaddrinuse(err error) bool {
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		if netErr.Op != "listen" {
			return false
		}
		return errors.Is(err, syscall.EADDRINUSE)
	}
	return false
}

// CanConnect returns true if the socket provided at the address
// is accepting new connections
func CanConnect(address string) bool {
	conn, err := AnonDialer(address, 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func hybridVsockDialer(addr string, port uint64, timeout time.Duration) (net.Conn, error) {
	timeoutCh := time.After(timeout)
	// Do 10 retries before timeout
	retryInterval := timeout / 10
	for {
		conn, err := net.DialTimeout("unix", addr, timeout)
		if err != nil {
			return nil, err
		}
		if _, err = fmt.Fprintln(conn, "CONNECT", port); err != nil {
			conn.Close()
			return nil, err
		}
		errChan := make(chan error, 1)
		go func() {
			reader := bufio.NewReader(conn)
			response, err := reader.ReadString('\n')
			if err != nil {
				errChan <- err
				return
			}
			if strings.Contains(response, "OK") {
				errChan <- nil
			} else {
				errChan <- fmt.Errorf("hybrid vsock handshake response error: %s", response)
			}
		}()
		select {
		case err = <-errChan:
			if err != nil {
				conn.Close()
				// When it is EOF, maybe the server side is not ready.
				if err == io.EOF {
					log.G(context.Background()).Warnf("Read hybrid vsock got EOF, server may not ready")
					time.Sleep(retryInterval)
					continue
				}
				return nil, err
			}
			return conn, nil
		case <-timeoutCh:
			conn.Close()
			return nil, fmt.Errorf("timeout waiting for hybrid vsocket handshake of %s:%d", addr, port)
		}
	}

}

func dialVsock(address string) (net.Conn, error) {
	contextIDString, portString, ok := strings.Cut(address, ":")
	if !ok {
		return nil, fmt.Errorf("invalid vsock address %s", address)
	}
	contextID, err := strconv.ParseUint(contextIDString, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vsock context id %s, %v", contextIDString, err)
	}
	if contextID > math.MaxUint32 {
		return nil, fmt.Errorf("vsock context id %d is invalid", contextID)
	}
	port, err := strconv.ParseUint(portString, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vsock port %s, %v", portString, err)
	}
	if port > math.MaxUint32 {
		return nil, fmt.Errorf("vsock port %d is invalid", port)
	}
	return vsock.Dial(uint32(contextID), uint32(port), &vsock.Config{})
}

func dialHybridVsock(address string, timeout time.Duration) (net.Conn, error) {
	addr, portString, ok := strings.Cut(address, ":")
	if !ok {
		return nil, fmt.Errorf("invalid hybrid vsock address %s", address)
	}
	port, err := strconv.ParseUint(portString, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse hybrid vsock port %s, %v", portString, err)
	}
	if port > math.MaxUint32 {
		return nil, fmt.Errorf("hybrid vsock port %d is invalid", port)
	}
	return hybridVsockDialer(addr, port, timeout)
}

func cleanupSockets(ctx context.Context) {
	if address, err := ReadAddress("address"); err == nil {
		_ = RemoveSocket(address)
	}
	if len(socketFlag) > 0 {
		_ = RemoveSocket("unix://" + socketFlag)
	} else if address, err := SocketAddress(ctx, addressFlag, id, false); err == nil {
		_ = RemoveSocket(address)
	}
	if len(debugSocketFlag) > 0 {
		_ = RemoveSocket("unix://" + debugSocketFlag)
	} else if address, err := SocketAddress(ctx, addressFlag, id, true); err == nil {
		_ = RemoveSocket(address)
	}
}
