// +build linux freebsd

package libnetwork

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/pkg/stringid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const (
	execSubdir      = "libnetwork"
	defaultExecRoot = "/run/docker"
	success         = "success"
)

// processSetKeyReexec is a private function that must be called only on an reexec path
// It expects 3 args { [0] = "libnetwork-setkey", [1] = <container-id>, [2] = <short-controller-id> }
// It also expects specs.State as a json string in <stdin>
// Refer to https://github.com/opencontainers/runc/pull/160/ for more information
// The docker exec-root can be specified as "-exec-root" flag. The default value is "/run/docker".
func processSetKeyReexec() {
	var err error

	// Return a failure to the calling process via ExitCode
	defer func() {
		if err != nil {
			logrus.Fatalf("%v", err)
		}
	}()

	execRoot := flag.String("exec-root", defaultExecRoot, "docker exec root")
	flag.Parse()

	// expecting 3 os.Args {[0]="libnetwork-setkey", [1]=<container-id>, [2]=<short-controller-id> }
	// (i.e. expecting 2 flag.Args())
	args := flag.Args()
	if len(args) < 2 {
		err = fmt.Errorf("Re-exec expects 2 args (after parsing flags), received : %d", len(args))
		return
	}
	containerID, shortCtlrID := args[0], args[1]

	// We expect specs.State as a json string in <stdin>
	stateBuf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return
	}
	var state specs.State
	if err = json.Unmarshal(stateBuf, &state); err != nil {
		return
	}

	err = SetExternalKey(shortCtlrID, containerID, fmt.Sprintf("/proc/%d/ns/net", state.Pid), *execRoot)
}

// SetExternalKey provides a convenient way to set an External key to a sandbox
func SetExternalKey(shortCtlrID string, containerID string, key string, execRoot string) error {
	keyData := setKeyData{
		ContainerID: containerID,
		Key:         key}

	uds := filepath.Join(execRoot, execSubdir, shortCtlrID+".sock")
	c, err := net.Dial("unix", uds)
	if err != nil {
		return err
	}
	defer c.Close()

	if err = sendKey(c, keyData); err != nil {
		return fmt.Errorf("sendKey failed with : %v", err)
	}
	return processReturn(c)
}

func sendKey(c net.Conn, data setKeyData) error {
	var err error
	defer func() {
		if err != nil {
			c.Close()
		}
	}()

	var b []byte
	if b, err = json.Marshal(data); err != nil {
		return err
	}

	_, err = c.Write(b)
	return err
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

func (c *controller) startExternalKeyListener() error {
	execRoot := defaultExecRoot
	if v := c.Config().Daemon.ExecRoot; v != "" {
		execRoot = v
	}
	udsBase := filepath.Join(execRoot, execSubdir)
	if err := os.MkdirAll(udsBase, 0600); err != nil {
		return err
	}
	shortCtlrID := stringid.TruncateID(c.id)
	uds := filepath.Join(udsBase, shortCtlrID+".sock")
	l, err := net.Listen("unix", uds)
	if err != nil {
		return err
	}
	if err := os.Chmod(uds, 0600); err != nil {
		l.Close()
		return err
	}
	c.Lock()
	c.extKeyListener = l
	c.Unlock()

	go c.acceptClientConnections(uds, l)
	return nil
}

func (c *controller) acceptClientConnections(sock string, l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			if _, err1 := os.Stat(sock); os.IsNotExist(err1) {
				logrus.Debugf("Unix socket %s doesn't exist. cannot accept client connections", sock)
				return
			}
			logrus.Errorf("Error accepting connection %v", err)
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
				logrus.Errorf("Error returning to the client %v", err)
			}
		}()
	}
}

func (c *controller) processExternalKey(conn net.Conn) error {
	buf := make([]byte, 1280)
	nr, err := conn.Read(buf)
	if err != nil {
		return err
	}
	var s setKeyData
	if err = json.Unmarshal(buf[0:nr], &s); err != nil {
		return err
	}

	var sandbox Sandbox
	search := SandboxContainerWalker(&sandbox, s.ContainerID)
	c.WalkSandboxes(search)
	if sandbox == nil {
		return types.BadRequestErrorf("no sandbox present for %s", s.ContainerID)
	}

	return sandbox.SetKey(s.Key)
}

func (c *controller) stopExternalKeyListener() {
	c.extKeyListener.Close()
}
