// +build !windows

package sockets

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func dialSSH(user, addr, socketPath string) (net.Conn, error) {
	ssh := "ssh" // TODO(AkihiroSuda): allow getenv("DOCKER_SSH")
	tmpDir, err := ioutil.TempDir("", "go-connections-ssh")
	if err != nil {
		return nil, err
	}
	tmpDirSocketPath := filepath.Join(tmpDir, "go-connections-ssh.sock")
	sshCommand, err := makeSSHCommand(ssh, user, addr, socketPath, tmpDirSocketPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}
	sshCmd := exec.Command(sshCommand[0], sshCommand[1:]...)
	sshCmd.Env = os.Environ()
	if err = sshCmd.Start(); err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}
	unixConn, err := makeUnixConn(tmpDirSocketPath)
	if err != nil {
		sshCmd.Process.Kill()
		os.RemoveAll(tmpDir)
		return nil, err
	}
	return &sshConn{
		Conn:             unixConn,
		tmpDir:           tmpDir,
		tmpDirSocketPath: tmpDirSocketPath,
		sshCmd:           sshCmd,
	}, nil
}

func makeUnixConn(localSocketPath string) (net.Conn, error) {
	retries := 10
	interval := 500 * time.Millisecond
	var (
		conn net.Conn
		err  error
	)
	for i := 0; i < retries; i++ {
		conn, err = net.DialUnix("unix", nil, &net.UnixAddr{Name: localSocketPath, Net: "unix"})
		if err == nil {
			return conn, err
		}
		time.Sleep(interval)
	}
	return conn, err
}

type sshConn struct {
	net.Conn
	tmpDir           string
	tmpDirSocketPath string
	sshCmd           *exec.Cmd
}

func (c *sshConn) Close() error {
	err := c.Conn.Close()
	c.sshCmd.Process.Kill()
	os.RemoveAll(c.tmpDir)
	return err
}

func makeSSHCommand(ssh, user, addr, remoteSocketPath, localSocketPath string) ([]string, error) {
	if ssh == "" || user == "" || addr == "" || remoteSocketPath == "" || localSocketPath == "" {
		return nil, errors.New("invalid ssh spec")
	}
	port := 22
	host := ""
	if addrSplit := strings.Split(addr, ":"); len(addrSplit) == 2 {
		host = addrSplit[0]
		var err error
		port, err = strconv.Atoi(addrSplit[1])
		if err != nil {
			return nil, err
		}
	} else if len(addrSplit) == 1 {
		host = addr
	} else {
		return nil, fmt.Errorf("invalid addr: %q", addr)
	}
	if host == "" {
		return nil, fmt.Errorf("invalid host: %q", host)
	}
	return []string{
		ssh,
		"-L", localSocketPath + ":" + remoteSocketPath,
		"-N",
		"-l", user,
		"-p", strconv.Itoa(port),
		host,
	}, nil
}
