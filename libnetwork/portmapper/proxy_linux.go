package portmapper

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

const userlandProxyCommandName = "docker-proxy"

func newProxyCommand(proto string, hostIP net.IP, hostPort int, containerIP net.IP, containerPort int, proxyPath string) (userlandProxy, error) {
	path := proxyPath
	if proxyPath == "" {
		cmd, err := exec.LookPath(userlandProxyCommandName)
		if err != nil {
			return nil, err
		}
		path = cmd
	}

	args := []string{
		path,
		"-proto", proto,
		"-host-ip", hostIP.String(),
		"-host-port", strconv.Itoa(hostPort),
		"-container-ip", containerIP.String(),
		"-container-port", strconv.Itoa(containerPort),
	}

	return &proxyCommand{
		cmd: &exec.Cmd{
			Path: path,
			Args: args,
			SysProcAttr: &syscall.SysProcAttr{
				Pdeathsig: syscall.SIGTERM, // send a sigterm to the proxy if the daemon process dies
			},
		},
	}, nil
}

// proxyCommand wraps an exec.Cmd to run the userland TCP and UDP
// proxies as separate processes.
type proxyCommand struct {
	waitMu     sync.Mutex
	waitCond   sync.Cond
	exitedWait bool
	waitError  error
	cmd        *exec.Cmd
}

func (p *proxyCommand) Start() error {
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("proxy unable to open os.Pipe %s", err)
	}
	defer r.Close()
	p.cmd.ExtraFiles = []*os.File{w}
	if err := p.cmd.Start(); err != nil {
		return err
	}
	w.Close()

	go func() {
		p.waitMu.Lock()
		if p.waitCond.L == nil {
			p.waitCond.L = &p.waitMu
		}
		p.waitError = p.cmd.Wait()
		p.waitCond.Broadcast()
		p.exitedWait = true
		p.waitMu.Unlock()
		if p.waitError != nil {
			logrus.Warnf("docker-proxy pid:%d run error: %v", p.cmd.Process.Pid, p.waitError)
		} else {
			logrus.Debugf("docker-proxy pid:%d exited successfully", p.cmd.Process.Pid)
		}
	}()

	errchan := make(chan error, 1)
	go func() {
		buf := make([]byte, 2)
		r.Read(buf)

		if string(buf) != "0\n" {
			errStr, err := io.ReadAll(r)
			if err != nil {
				errchan <- fmt.Errorf("Error reading exit status from userland proxy: %v", err)
				return
			}

			errchan <- fmt.Errorf("Error starting userland proxy: %s", errStr)
			return
		}
		errchan <- nil
	}()

	select {
	case err := <-errchan:
		return err
	case <-time.After(16 * time.Second):
		return fmt.Errorf("Timed out proxy starting the userland proxy")
	}
}

func (p *proxyCommand) Stop() error {
	if p.cmd.Process != nil {
		if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
			return err
		}
		p.waitMu.Lock()
		if p.waitCond.L == nil {
			p.waitCond.L = &p.waitMu
		}
		for !p.exitedWait {
			p.waitCond.Wait()
		}
		p.waitMu.Unlock()
		if p.waitError != nil {
			logrus.Warnf("docker-proxy pid:%d stop error: %v", p.cmd.Process.Pid, p.waitError)
		} else {
			logrus.Debugf("docker-proxy pid:%d stop successfully", p.cmd.Process.Pid)
		}
		return p.waitError
	}
	return nil
}
