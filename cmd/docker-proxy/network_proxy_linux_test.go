//go:build !windows

package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ishidawataru/sctp"
	"gotest.tools/v3/assert"
)

var (
	testBuf     = []byte("Buffalo1 buffalo2 Buffalo3 buffalo4 buffalo5 buffalo6 Buffalo7 buffalo8")
	testBufSize = len(testBuf)
)

type EchoServer interface {
	Run()
	Close()
	LocalAddr() net.Addr
}

type EchoServerOptions struct {
	TCPHalfClose bool
}

type StreamEchoServer struct {
	listener net.Listener
	testCtx  *testing.T
	opts     EchoServerOptions
}

type UDPEchoServer struct {
	conn    net.PacketConn
	testCtx *testing.T
}

const hopefullyFreePort = 25587

func NewEchoServer(t *testing.T, proto, address string, opts EchoServerOptions) EchoServer {
	var server EchoServer
	if !strings.HasPrefix(proto, "tcp") && opts.TCPHalfClose {
		t.Fatalf("TCPHalfClose is not supported for %s", proto)
	}

	switch {
	case strings.HasPrefix(proto, "tcp"):
		listener, err := net.Listen(proto, address)
		if err != nil {
			t.Fatal(err)
		}
		server = &StreamEchoServer{listener: listener, testCtx: t, opts: opts}
	case strings.HasPrefix(proto, "udp"):
		socket, err := net.ListenPacket(proto, address)
		if err != nil {
			t.Fatal(err)
		}
		server = &UDPEchoServer{conn: socket, testCtx: t}
	case strings.HasPrefix(proto, "sctp"):
		addr, err := sctp.ResolveSCTPAddr(proto, address)
		if err != nil {
			t.Fatal(err)
		}
		listener, err := sctp.ListenSCTP(proto, addr)
		if err != nil {
			t.Fatal(err)
		}
		server = &StreamEchoServer{listener: listener, testCtx: t}
	default:
		t.Fatalf("unknown protocol: %s", proto)
	}
	return server
}

func (server *StreamEchoServer) Run() {
	go func() {
		for {
			client, err := server.listener.Accept()
			if err != nil {
				return
			}
			go func(client net.Conn) {
				if server.opts.TCPHalfClose {
					data, err := io.ReadAll(client)
					if err != nil {
						server.testCtx.Logf("io.ReadAll() failed for the client: %v\n", err.Error())
					}
					if _, err := client.Write(data); err != nil {
						server.testCtx.Logf("can't echo to the client: %v\n", err.Error())
					}
					client.(*net.TCPConn).CloseWrite()
				} else {
					if _, err := io.Copy(client, client); err != nil {
						server.testCtx.Logf("can't echo to the client: %v\n", err.Error())
					}
					client.Close()
				}
			}(client)
		}
	}()
}

func (server *StreamEchoServer) LocalAddr() net.Addr { return server.listener.Addr() }
func (server *StreamEchoServer) Close()              { server.listener.Close() }

func (server *UDPEchoServer) Run() {
	go func() {
		readBuf := make([]byte, 1024)
		for {
			read, from, err := server.conn.ReadFrom(readBuf)
			if err != nil {
				return
			}
			for i := 0; i != read; {
				written, err := server.conn.WriteTo(readBuf[i:read], from)
				if err != nil {
					break
				}
				i += written
			}
		}
	}()
}

func (server *UDPEchoServer) LocalAddr() net.Addr { return server.conn.LocalAddr() }
func (server *UDPEchoServer) Close()              { server.conn.Close() }

func tcpListener(t *testing.T, nw string, addr *net.TCPAddr) (*os.File, *net.TCPAddr) {
	t.Helper()
	l, err := net.ListenTCP(nw, addr)
	assert.NilError(t, err)
	osFile, err := l.File()
	assert.NilError(t, err)
	tcpAddr := l.Addr().(*net.TCPAddr)
	err = l.Close()
	assert.NilError(t, err)
	return osFile, tcpAddr
}

func udpListener(t *testing.T, nw string, addr *net.UDPAddr) (*os.File, *net.UDPAddr) {
	t.Helper()
	l, err := net.ListenUDP(nw, addr)
	assert.NilError(t, err)
	osFile, err := l.File()
	assert.NilError(t, err)
	err = l.Close()
	assert.NilError(t, err)
	return osFile, l.LocalAddr().(*net.UDPAddr)
}

func testProxyAt(t *testing.T, proto string, proxy Proxy, addr string, halfClose bool) {
	t.Helper()
	defer proxy.Close()
	go proxy.Run()
	var client net.Conn
	var err error
	if strings.HasPrefix(proto, "sctp") {
		var a *sctp.SCTPAddr
		a, err = sctp.ResolveSCTPAddr(proto, addr)
		if err != nil {
			t.Fatal(err)
		}
		client, err = sctp.DialSCTP(proto, nil, a)
	} else {
		client, err = net.Dial(proto, addr)
	}

	if err != nil {
		t.Fatalf("Can't connect to the proxy: %v", err)
	}
	defer client.Close()
	client.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err = client.Write(testBuf); err != nil {
		t.Fatal(err)
	}
	if halfClose {
		if proto != "tcp" {
			t.Fatalf("halfClose is not supported for %s", proto)
		}
		client.(*net.TCPConn).CloseWrite()
	}
	recvBuf := make([]byte, testBufSize)
	if _, err = client.Read(recvBuf); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(testBuf, recvBuf) {
		t.Fatal(fmt.Errorf("Expected [%v] but got [%v]", testBuf, recvBuf))
	}
}

func testTCP4Proxy(t *testing.T, halfClose bool, hostPort int) {
	t.Helper()
	backend := NewEchoServer(t, "tcp", "127.0.0.1:0", EchoServerOptions{TCPHalfClose: halfClose})
	defer backend.Close()
	backend.Run()
	backendAddr := backend.LocalAddr().(*net.TCPAddr)
	var listener *os.File
	frontendAddr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	if hostPort == 0 {
		listener, frontendAddr = tcpListener(t, "tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	} else {
		frontendAddr.Port = hostPort
	}
	config := ProxyConfig{
		Proto:         "tcp",
		HostIP:        frontendAddr.IP,
		HostPort:      frontendAddr.Port,
		ContainerIP:   backendAddr.IP,
		ContainerPort: backendAddr.Port,
		ListenSock:    listener,
	}
	proxy, err := newProxy(config)
	if err != nil {
		t.Fatal(err)
	}
	testProxyAt(t, "tcp", proxy, frontendAddr.String(), halfClose)
}

func TestTCP4Proxy(t *testing.T) {
	testTCP4Proxy(t, false, 0)
}

func TestTCP4ProxyNoListener(t *testing.T) {
	testTCP4Proxy(t, false, hopefullyFreePort)
}

func TestTCP4ProxyHalfClose(t *testing.T) {
	testTCP4Proxy(t, true, 0)
}

func TestTCP6Proxy(t *testing.T) {
	backend := NewEchoServer(t, "tcp", "[::1]:0", EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	backendAddr := backend.LocalAddr().(*net.TCPAddr)
	listener, frontendAddr := tcpListener(t, "tcp6", &net.TCPAddr{IP: net.IPv6loopback, Port: 0})
	config := ProxyConfig{
		Proto:         "tcp",
		HostIP:        frontendAddr.IP,
		HostPort:      frontendAddr.Port,
		ContainerIP:   backendAddr.IP,
		ContainerPort: backendAddr.Port,
		ListenSock:    listener,
	}
	proxy, err := newProxy(config)
	if err != nil {
		t.Fatal(err)
	}
	testProxyAt(t, "tcp", proxy, frontendAddr.String(), false)
}

func TestTCPDualStackProxy(t *testing.T) {
	backend := NewEchoServer(t, "tcp", "[::1]:0", EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	backendAddr := backend.LocalAddr().(*net.TCPAddr)
	listener, frontendAddr := tcpListener(t, "tcp", &net.TCPAddr{IP: net.IPv6zero, Port: 0})
	config := ProxyConfig{
		Proto:         "tcp",
		HostIP:        frontendAddr.IP,
		HostPort:      frontendAddr.Port,
		ContainerIP:   backendAddr.IP,
		ContainerPort: backendAddr.Port,
		ListenSock:    listener,
	}
	proxy, err := newProxy(config)
	if err != nil {
		t.Fatal(err)
	}
	ipv4ProxyAddr := &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: frontendAddr.Port,
	}
	testProxyAt(t, "tcp", proxy, ipv4ProxyAddr.String(), false)
}

func testUDP4Proxy(t *testing.T, hostPort int) {
	t.Helper()
	backend := NewEchoServer(t, "udp", "127.0.0.1:0", EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	var listener *os.File
	frontendAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	if hostPort == 0 {
		listener, frontendAddr = udpListener(t, "udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	} else {
		frontendAddr.Port = hostPort
	}
	backendAddr := backend.LocalAddr().(*net.UDPAddr)
	config := ProxyConfig{
		Proto:         "udp",
		HostIP:        frontendAddr.IP,
		HostPort:      frontendAddr.Port,
		ContainerIP:   backendAddr.IP,
		ContainerPort: backendAddr.Port,
		ListenSock:    listener,
	}
	proxy, err := newProxy(config)
	if err != nil {
		t.Fatal(err)
	}
	testProxyAt(t, "udp", proxy, frontendAddr.String(), false)
}

func TestUDP4Proxy(t *testing.T) {
	testUDP4Proxy(t, 0)
}

func TestUDP4ProxyNoListener(t *testing.T) {
	testUDP4Proxy(t, hopefullyFreePort)
}

func TestUDP6Proxy(t *testing.T) {
	backend := NewEchoServer(t, "udp", "[::1]:0", EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	listener, frontendAddr := udpListener(t, "udp6", &net.UDPAddr{IP: net.IPv6loopback, Port: 0})
	backendAddr := backend.LocalAddr().(*net.UDPAddr)
	config := ProxyConfig{
		Proto:         "udp",
		HostIP:        frontendAddr.IP,
		HostPort:      frontendAddr.Port,
		ContainerIP:   backendAddr.IP,
		ContainerPort: backendAddr.Port,
		ListenSock:    listener,
	}
	proxy, err := newProxy(config)
	if err != nil {
		t.Fatal(err)
	}
	testProxyAt(t, "udp", proxy, frontendAddr.String(), false)
}

func TestUDPWriteError(t *testing.T) {
	frontendAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	// Hopefully, this port will be free: */
	backendAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: hopefullyFreePort}
	listener, frontendAddr := udpListener(t, "udp4", frontendAddr)
	config := ProxyConfig{
		Proto:         "udp",
		HostIP:        frontendAddr.IP,
		HostPort:      frontendAddr.Port,
		ContainerIP:   backendAddr.IP,
		ContainerPort: backendAddr.Port,
		ListenSock:    listener,
	}
	proxy, err := newProxy(config)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	go proxy.Run()
	client, err := net.Dial("udp", frontendAddr.String())
	if err != nil {
		t.Fatalf("Can't connect to the proxy: %v", err)
	}
	defer client.Close()
	// Make sure the proxy doesn't stop when there is no actual backend:
	client.Write(testBuf)
	client.Write(testBuf)
	backend := NewEchoServer(t, "udp", backendAddr.String(), EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	client.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err = client.Write(testBuf); err != nil {
		t.Fatal(err)
	}
	recvBuf := make([]byte, testBufSize)
	if _, err = client.Read(recvBuf); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(testBuf, recvBuf) {
		t.Fatal(fmt.Errorf("Expected [%v] but got [%v]", testBuf, recvBuf))
	}
}

func TestSCTP4ProxyNoListener(t *testing.T) {
	backend := NewEchoServer(t, "sctp", "127.0.0.1:0", EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	backendAddr := backend.LocalAddr().(*sctp.SCTPAddr)
	config := ProxyConfig{
		Proto:         "sctp",
		HostIP:        net.IPv4(127, 0, 0, 1),
		HostPort:      hopefullyFreePort,
		ContainerIP:   backendAddr.IPAddrs[0].IP,
		ContainerPort: backendAddr.Port,
	}
	proxy, err := newProxy(config)
	assert.NilError(t, err)
	testProxyAt(t, "sctp", proxy, fmt.Sprintf("%s:%d", config.HostIP, config.HostPort), false)
}

func TestSCTP6ProxyNoListener(t *testing.T) {
	backend := NewEchoServer(t, "sctp", "[::1]:0", EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	backendAddr := backend.LocalAddr().(*sctp.SCTPAddr)
	config := ProxyConfig{
		Proto:         "sctp",
		HostIP:        net.IPv6loopback,
		HostPort:      hopefullyFreePort,
		ContainerIP:   backendAddr.IPAddrs[0].IP,
		ContainerPort: backendAddr.Port,
	}
	proxy, err := newProxy(config)
	assert.NilError(t, err)
	testProxyAt(t, "sctp", proxy, fmt.Sprintf("[%s]:%d", config.HostIP, config.HostPort), false)
}
