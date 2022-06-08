//go:build linux || freebsd

package libnetwork

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/stringid"
)

const (
	execSubdir      = "libnetwork"
	defaultExecRoot = "/var/run/docker"
	success         = "success"
)

func init() {
	// TODO(thaJeztah): should this actually be registered on FreeBSD, or only on Linux?
	reexec.Register("libnetwork-setkey", processSetKeyReexec)
}

// shallowState holds information about the runtime state of the container.
// It's a reduced version of github.com/opencontainers/runtime-spec Spec
// to only contain the field(s) we're interested in.
type shallowState struct {
	// ID is the container ID
	ID string `json:"id"`
	// Pid is the process ID for the container process.
	Pid int `json:"pid,omitempty"`
}

// processSetKeyReexec is a private function that must be called only on an reexec path
// It expects 3 args { [0] = "libnetwork-setkey", [1] = <container-id>, [2] = <short-controller-id> }
// It also expects specs.State as a json string in <stdin>
// Refer to https://github.com/opencontainers/runc/pull/160/ for more information
// The docker exec-root can be specified as "-exec-root" flag. The default value is "/run/docker".
func processSetKeyReexec() {
	if err := setKey(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setKey() error {
	sockPath := flag.String("sock", "", "libnetwork controller socket path")
	flag.Parse()

	if *sockPath == "" {
		return fmt.Errorf("libnetwork-setkey: missing '-sock' option")
	}

	// OCI runtime hooks send specs.State as a json string in <stdin>
	var state shallowState
	if err := json.NewDecoder(os.Stdin).Decode(&state); err != nil {
		return err
	}

	return setExternalKey(*sockPath, state.ID, fmt.Sprintf("/proc/%d/ns/net", state.Pid))
}

// setExternalKey provides a convenient way to set an External key to a sandbox
func setExternalKey(sockPath string, containerID string, key string) error {
	c, err := net.Dial("unix", sockPath)
	if err != nil {
		return err
	}
	defer c.Close()

	_, err = c.Write([]byte(containerID + "=" + key))
	if err != nil {
		return fmt.Errorf("sendKey failed: %v", err)
	}
	return processReturn(c)
}

func processReturn(r io.Reader) error {
	buf := make([]byte, 1024)
	n, err := r.Read(buf[:])
	if err != nil {
		return fmt.Errorf("failed to read buf in processReturn: %v", err)
	}
	if string(buf[0:n]) != success {
		return fmt.Errorf(string(buf[0:n]))
	}
	return nil
}

func (c *Controller) startExternalKeyListener() error {
	// TODO(thaJeztah) should this be an error-condition if we don't have the daemon's actual exec-root?
	execRoot := defaultExecRoot
	if v := c.Config().ExecRoot; v != "" {
		execRoot = v
	}
	udsBase := filepath.Join(execRoot, execSubdir)
	if err := os.MkdirAll(udsBase, 0o600); err != nil {
		return err
	}
	uds := filepath.Join(udsBase, stringid.TruncateID(c.id)+".sock")
	l, err := net.Listen("unix", uds)
	if err != nil {
		return err
	}
	if err := os.Chmod(uds, 0o600); err != nil {
		l.Close()
		return err
	}
	c.mu.Lock()
	c.controlSocket = uds
	c.extKeyListener = l
	c.mu.Unlock()

	go c.acceptClientConnections(uds, l)
	return nil
}

func (c *Controller) acceptClientConnections(sock string, l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			if _, err1 := os.Stat(sock); os.IsNotExist(err1) {
				log.G(context.TODO()).Debugf("Unix socket %s doesn't exist. cannot accept client connections", sock)
				return
			}
			log.G(context.TODO()).Errorf("Error accepting connection %v", err)
			continue
		}
		go func() {
			defer conn.Close()

			err := c.processExternalKey(conn)
			ret := success
			if err != nil {
				ret = err.Error()
			}

			_, err = conn.Write([]byte(ret))
			if err != nil {
				log.G(context.TODO()).Errorf("Error returning to the client %v", err)
			}
		}()
	}
}

func (c *Controller) processExternalKey(conn net.Conn) error {
	buf := make([]byte, 1280)
	nr, err := conn.Read(buf)
	if err != nil {
		return err
	}

	parts := bytes.SplitN(buf[0:nr], []byte("="), 2)
	if len(parts) != 2 {
		return types.InvalidParameterErrorf("invalid key data (%s): should be formatted as <container-ID>=<key>", string(buf[0:nr]))
	}

	containerID, key := string(parts[0]), string(parts[1])
	sb, err := c.GetSandbox(containerID)
	if err != nil {
		return types.InvalidParameterErrorf("failed to get sandbox for %s", containerID)
	}
	return sb.SetKey(key)
}

func (c *Controller) stopExternalKeyListener() {
	_ = c.extKeyListener.Close()
}
