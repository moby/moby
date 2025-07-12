package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon/libnetwork/portmapperapi"
	"github.com/docker/docker/daemon/libnetwork/types"
)

// ProxyManager is a struct that manages userland proxies.
type ProxyManager struct {
	ProxyPath string
}

type Proxy struct {
	p       *os.Process
	pidfd   int // pidfd might be -1 on systems that don't support it
	wait    chan error
	stopped *atomic.Bool
}

// StartProxy starts the proxy process. If listenSock is not nil, it must be a
// bound socket that can be passed to the proxy process for it to listen on.
func (pm ProxyManager) StartProxy(
	pb types.PortBinding,
	listenSock *os.File,
) (_ portmapperapi.Proxy, retErr error) {
	if pm.ProxyPath == "" {
		return nil, errors.New("no path provided for userland-proxy binary")
	}
	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("proxy unable to open os.Pipe %s", err)
	}
	defer func() {
		if w != nil {
			w.Close()
		}
		r.Close()
	}()

	var pidfd int
	cmd := &exec.Cmd{
		Path: pm.ProxyPath,
		Args: []string{
			pm.ProxyPath,
			"-proto", pb.Proto.String(),
			"-host-ip", pb.HostIP.String(),
			"-host-port", strconv.FormatUint(uint64(pb.HostPort), 10),
			"-container-ip", pb.IP.String(),
			"-container-port", strconv.FormatUint(uint64(pb.Port), 10),
		},
		ExtraFiles: []*os.File{w},
		SysProcAttr: &syscall.SysProcAttr{
			PidFD:     &pidfd,
			Pdeathsig: syscall.SIGTERM, // send a sigterm to the proxy if the creating thread in the daemon process dies (https://go.dev/issue/27505)
		},
	}
	if listenSock != nil {
		cmd.Args = append(cmd.Args, "-use-listen-fd")
		cmd.ExtraFiles = append(cmd.ExtraFiles, listenSock)
	}

	wait := make(chan error, 1)

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
	var stopped atomic.Bool
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		err := cmd.Start()
		started <- err
		if err != nil {
			return
		}
		err = cmd.Wait()
		if !stopped.Load() {
			log.G(context.Background()).WithFields(log.Fields{
				"proto":          pb.Proto,
				"host-ip":        pb.HostIP,
				"host-port":      pb.HostPort,
				"container-ip":   pb.IP,
				"container-port": pb.Port,
			}).Info("Userland proxy exited early (this is expected during daemon shutdown)")
		}
		wait <- err
	}()
	if err := <-started; err != nil {
		return nil, err
	}
	w.Close()
	w = nil

	errchan := make(chan error, 1)
	go func() {
		buf := make([]byte, 2)
		r.Read(buf)

		if string(buf) != "0\n" {
			errStr, err := io.ReadAll(r)
			if err != nil {
				errchan <- fmt.Errorf("error reading exit status from userland proxy: %v", err)
				return
			}
			// If the user has an old docker-proxy in their PATH, and we passed "-use-listen-fd"
			// on the command line, it exits with no response on the pipe.
			if listenSock != nil && buf[0] == 0 && len(errStr) == 0 {
				errchan <- errors.New("failed to start docker-proxy, check that the current version is in your $PATH")
				return
			}
			errchan <- fmt.Errorf("error starting userland proxy: %s", errStr)
			return
		}
		errchan <- nil
	}()

	select {
	case err := <-errchan:
		if err != nil {
			return nil, err
		}
	case <-time.After(16 * time.Second):
		return nil, errors.New("timed out starting the userland proxy")
	}

	return &Proxy{
		p:       cmd.Process,
		pidfd:   pidfd,
		wait:    wait,
		stopped: &stopped,
	}, nil
}

// Stop stops the proxy process. It cannot be called multiple times — it'll
// return a nil error on subsequent calls, irrespective of the original error.
func (p *Proxy) Stop() error {
	if p.p == nil || p.stopped.Load() {
		return nil
	}

	p.stopped.Store(true)
	if err := p.p.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}

	err := <-p.wait

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return err
	}

	wstatus := exitErr.Sys().(syscall.WaitStatus)
	if wstatus.Signal() != os.Interrupt || wstatus.ExitStatus() > 0 {
		return err
	}

	return nil
}
