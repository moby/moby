package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ishidawataru/sctp"
	"gotest.tools/v3/skip"
)

var (
	testBuf     = []byte("Buffalo buffalo Buffalo buffalo buffalo buffalo Buffalo buffalo")
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

func testProxyAt(t *testing.T, proto string, proxy Proxy, addr string, halfClose bool) {
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

func testProxy(t *testing.T, proto string, proxy Proxy, halfClose bool) {
	testProxyAt(t, proto, proxy, proxy.FrontendAddr().String(), halfClose)
}

func testTCP4Proxy(t *testing.T, halfClose bool) {
	backend := NewEchoServer(t, "tcp", "127.0.0.1:0", EchoServerOptions{TCPHalfClose: halfClose})
	defer backend.Close()
	backend.Run()
	frontendAddr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	proxy, err := NewProxy(frontendAddr, backend.LocalAddr())
	if err != nil {
		t.Fatal(err)
	}
	testProxy(t, "tcp", proxy, halfClose)
}

func TestTCP4Proxy(t *testing.T) {
	testTCP4Proxy(t, false)
}

func TestTCP4ProxyHalfClose(t *testing.T) {
	testTCP4Proxy(t, true)
}

func TestTCP6Proxy(t *testing.T) {
	t.Skip("Need to start CI docker with --ipv6")
	backend := NewEchoServer(t, "tcp", "[::1]:0", EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	frontendAddr := &net.TCPAddr{IP: net.IPv6loopback, Port: 0}
	proxy, err := NewProxy(frontendAddr, backend.LocalAddr())
	if err != nil {
		t.Fatal(err)
	}
	testProxy(t, "tcp", proxy, false)
}

func TestTCPDualStackProxy(t *testing.T) {
	// If I understand `godoc -src net favoriteAddrFamily` (used by the
	// net.Listen* functions) correctly this should work, but it doesn't.
	t.Skip("No support for dual stack yet")
	backend := NewEchoServer(t, "tcp", "[::1]:0", EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	frontendAddr := &net.TCPAddr{IP: net.IPv6loopback, Port: 0}
	proxy, err := NewProxy(frontendAddr, backend.LocalAddr())
	if err != nil {
		t.Fatal(err)
	}
	ipv4ProxyAddr := &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: proxy.FrontendAddr().(*net.TCPAddr).Port,
	}
	testProxyAt(t, "tcp", proxy, ipv4ProxyAddr.String(), false)
}

func TestUDP4Proxy(t *testing.T) {
	backend := NewEchoServer(t, "udp", "127.0.0.1:0", EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	frontendAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	proxy, err := NewProxy(frontendAddr, backend.LocalAddr())
	if err != nil {
		t.Fatal(err)
	}
	testProxy(t, "udp", proxy, false)
}

func TestUDP6Proxy(t *testing.T) {
	t.Skip("Need to start CI docker with --ipv6")
	backend := NewEchoServer(t, "udp", "[::1]:0", EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	frontendAddr := &net.UDPAddr{IP: net.IPv6loopback, Port: 0}
	proxy, err := NewProxy(frontendAddr, backend.LocalAddr())
	if err != nil {
		t.Fatal(err)
	}
	testProxy(t, "udp", proxy, false)
}

func TestUDPWriteError(t *testing.T) {
	frontendAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	// Hopefully, this port will be free: */
	backendAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 25587}
	proxy, err := NewProxy(frontendAddr, backendAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	go proxy.Run()
	client, err := net.Dial("udp", "127.0.0.1:25587")
	if err != nil {
		t.Fatalf("Can't connect to the proxy: %v", err)
	}
	defer client.Close()
	// Make sure the proxy doesn't stop when there is no actual backend:
	client.Write(testBuf)
	client.Write(testBuf)
	backend := NewEchoServer(t, "udp", "127.0.0.1:25587", EchoServerOptions{})
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

func TestSCTP4Proxy(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "sctp is not supported on windows")

	backend := NewEchoServer(t, "sctp", "127.0.0.1:0", EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	frontendAddr := &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.IPv4(127, 0, 0, 1)}}, Port: 0}
	proxy, err := NewProxy(frontendAddr, backend.LocalAddr())
	if err != nil {
		t.Fatal(err)
	}
	testProxy(t, "sctp", proxy, false)
}

func TestSCTP6Proxy(t *testing.T) {
	t.Skip("Need to start CI docker with --ipv6")
	skip.If(t, runtime.GOOS == "windows", "sctp is not supported on windows")

	backend := NewEchoServer(t, "sctp", "[::1]:0", EchoServerOptions{})
	defer backend.Close()
	backend.Run()
	frontendAddr := &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.IPv6loopback}}, Port: 0}
	proxy, err := NewProxy(frontendAddr, backend.LocalAddr())
	if err != nil {
		t.Fatal(err)
	}
	testProxy(t, "sctp", proxy, false)
}
