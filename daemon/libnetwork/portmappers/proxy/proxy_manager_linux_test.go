package proxy

import (
	"net"
	"os"
	"testing"

	"github.com/docker/docker/daemon/libnetwork/types"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

const proxyPath = "/usr/local/bin/docker-proxy"

func TestStartAndStopProxy(t *testing.T) {
	if _, err := os.Stat(proxyPath); err != nil {
		t.Skipf("proxy binary %s not found", proxyPath)
		return
	}

	pm := ProxyManager{ProxyPath: proxyPath}
	proxy, err := pm.StartProxy(types.PortBinding{
		Proto:    types.TCP,
		IP:       net.ParseIP("127.0.0.1"),
		Port:     0,
		HostIP:   net.ParseIP("127.0.0.1"),
		HostPort: 61234,
	}, nil)
	assert.NilError(t, err)

	p := proxy.(*Proxy)

	if p.pidfd == -1 {
		_ = proxy.Stop()
		t.Skip("pidfd not supported on this system")
		return
	}

	pidfd := os.NewFile(uintptr(p.pidfd), "")
	assert.Assert(t, isRunning(pidfd))

	err = proxy.Stop()
	assert.NilError(t, err)
	assert.Assert(t, !isRunning(pidfd))
}

func TestKillProxy(t *testing.T) {
	if _, err := os.Stat(proxyPath); err != nil {
		t.Skipf("proxy binary %s not found", proxyPath)
		return
	}

	pm := ProxyManager{ProxyPath: proxyPath}
	proxy, err := pm.StartProxy(types.PortBinding{
		Proto:    types.TCP,
		IP:       net.ParseIP("127.0.0.1"),
		Port:     0,
		HostIP:   net.ParseIP("127.0.0.1"),
		HostPort: 61234,
	}, nil)
	assert.NilError(t, err)

	p := proxy.(*Proxy)

	if p.pidfd == -1 {
		_ = proxy.Stop()
		t.Skip("pidfd not supported on this system")
		return
	}

	pidfd := os.NewFile(uintptr(p.pidfd), "")
	assert.Assert(t, isRunning(pidfd))

	// Kill the proxy process and verify that Proxy.Stop() returns an error
	err = p.p.Kill()
	assert.NilError(t, err)

	err = proxy.Stop()
	assert.ErrorContains(t, err, "signal: killed")
	assert.Assert(t, !isRunning(pidfd))
}

func isRunning(pidfd *os.File) bool {
	err := unix.Waitid(unix.P_PIDFD, int(pidfd.Fd()), nil, unix.WEXITED|unix.WNOHANG, nil)
	return err == nil
}
