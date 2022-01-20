//go:build linux || freebsd

package libnetwork

import (
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
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	execSubdir      = "libnetwork"
	defaultExecRoot = "/run/docker"
	success         = "success"
)

func init() {
	// TODO(thaJeztah): should this actually be registered on FreeBSD, or only on Linux?
	reexec.Register("libnetwork-setkey", processSetKeyReexec)
}

type setKeyData struct {
	ContainerID string
	Key         string
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
	execRoot := flag.String("exec-root", defaultExecRoot, "docker exec root")
	flag.Parse()

	// expecting 3 os.Args {[0]="libnetwork-setkey", [1]=<container-id>, [2]=<short-controller-id> }
	// (i.e. expecting 2 flag.Args())
	args := flag.Args()
	if len(args) < 2 {
		return fmt.Errorf("re-exec expects 2 args (after parsing flags), received : %d", len(args))
	}
	containerID, shortCtlrID := args[0], args[1]

	// We expect specs.State as a json string in <stdin>
	var state specs.State
	if err := json.NewDecoder(os.Stdin).Decode(&state); err != nil {
		return err
	}

	return SetExternalKey(shortCtlrID, containerID, fmt.Sprintf("/proc/%d/ns/net", state.Pid), *execRoot)
}

// SetExternalKey provides a convenient way to set an External key to a sandbox
func SetExternalKey(shortCtlrID string, containerID string, key string, execRoot string) error {
	uds := filepath.Join(execRoot, execSubdir, shortCtlrID+".sock")
	c, err := net.Dial("unix", uds)
	if err != nil {
		return err
	}
	defer c.Close()

	err = json.NewEncoder(c).Encode(setKeyData{
		ContainerID: containerID,
		Key:         key,
	})
	if err != nil {
		return fmt.Errorf("sendKey failed with : %v", err)
	}
	return processReturn(c)
}

func processReturn(r io.Reader) error {
	buf := make([]byte, 1024)
	n, err := r.Read(buf[:])
	if err != nil {
		return fmt.Errorf("failed to read buf in processReturn : %v", err)
	}
	if string(buf[0:n]) != success {
		return fmt.Errorf(string(buf[0:n]))
	}
	return nil
}

func (c *Controller) startExternalKeyListener() error {
	execRoot := defaultExecRoot
	if v := c.Config().ExecRoot; v != "" {
		execRoot = v
	}
	udsBase := filepath.Join(execRoot, execSubdir)
	if err := os.MkdirAll(udsBase, 0o600); err != nil {
		return err
	}
	shortCtlrID := stringid.TruncateID(c.id)
	uds := filepath.Join(udsBase, shortCtlrID+".sock")
	l, err := net.Listen("unix", uds)
	if err != nil {
		return err
	}
	if err := os.Chmod(uds, 0o600); err != nil {
		l.Close()
		return err
	}
	c.mu.Lock()
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
	var s setKeyData
	if err = json.Unmarshal(buf[0:nr], &s); err != nil {
		return err
	}

	var sandbox *Sandbox
	search := SandboxContainerWalker(&sandbox, s.ContainerID)
	c.WalkSandboxes(search)
	if sandbox == nil {
		return types.BadRequestErrorf("no sandbox present for %s", s.ContainerID)
	}

	return sandbox.SetKey(s.Key)
}

func (c *Controller) stopExternalKeyListener() {
	c.extKeyListener.Close()
}
