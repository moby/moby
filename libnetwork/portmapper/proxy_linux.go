package portmapper

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
	"time"
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
				Pdeathsig: syscall.SIGTERM, // send a sigterm to the proxy if the creating thread in the daemon process dies (https://go.dev/issue/27505)
			},
		},
		wait: make(chan error, 1),
	}, nil
}

// proxyCommand wraps an exec.Cmd to run the userland TCP and UDP
// proxies as separate processes.
type proxyCommand struct {
	cmd  *exec.Cmd
	wait chan error
}

func (p *proxyCommand) Start() error {
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("proxy unable to open os.Pipe %s", err)
	}
	defer r.Close()
	p.cmd.ExtraFiles = []*os.File{w}

	// As p.cmd.SysProcAttr.Pdeathsig is set, the signal will be sent to the
	// process when the OS thread on which p.cmd.Start() was executed dies.
	// If the thread is allowed to be released back into the goroutine
	// thread pool, the thread could get terminated at any time if a
	// goroutine gets scheduled onto it which calls runtime.LockOSThread()
	// and exits without a matching number of runtime.UnlockOSThread()
	// calls. Ensure that the thread from which Start() is called stays
	// alive until the proxy or the daemon process exits to prevent the
	// proxy from getting terminated early. See https://go.dev/issue/27505
	// for more details.
	started := make(chan error)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		err := p.cmd.Start()
		started <- err
		if err != nil {
			return
		}
		p.wait <- p.cmd.Wait()
	}()
	if err := <-started; err != nil {
		return err
	}
	w.Close()

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
		return <-p.wait
	}
	return nil
}
