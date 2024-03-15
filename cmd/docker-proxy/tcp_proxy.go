package main

import (
	"io"
	"log"
	"net"
	"time"
)

// TCPProxy is a proxy for TCP connections. It implements the Proxy interface to
// handle TCP traffic forwarding between the frontend and backend addresses.
type TCPProxy struct {
	listener     *net.TCPListener
	frontendAddr *net.TCPAddr
	backendAddr  *net.TCPAddr
}

// NewTCPProxy creates a new TCPProxy.
func NewTCPProxy(frontendAddr, backendAddr *net.TCPAddr) (*TCPProxy, error) {
	// detect version of hostIP to bind only to correct version
	ipVersion := ipv4
	if frontendAddr.IP.To4() == nil {
		ipVersion = ipv6
	}
	listener, err := net.ListenTCP("tcp"+string(ipVersion), frontendAddr)
	if err != nil {
		return nil, err
	}
	// If the port in frontendAddr was 0 then ListenTCP will have a picked
	// a port to listen on, hence the call to Addr to get that actual port:
	return &TCPProxy{
		listener:     listener,
		frontendAddr: listener.Addr().(*net.TCPAddr),
		backendAddr:  backendAddr,
	}, nil
}

func (proxy *TCPProxy) clientLoop(client *net.TCPConn) {
	conn, err := net.DialTimeout("tcp", proxy.backendAddr.String(), 5*time.Second)
	if err != nil {
		client.Close()
		log.Printf("Can't forward traffic to backend tcp/%v: %s\n", proxy.backendAddr, err)
		return
	}
	backend := conn.(*net.TCPConn)

	done := make(chan struct{}, 2)

	broker := func(to, from *net.TCPConn) {
		io.Copy(to, from)
		done <- struct{}{}
	}

	go broker(client, backend)
	go broker(backend, client)

	<-done

	// reduce close_wait states
	backend.SetLinger(0)
	backend.SetDeadline(time.Now().Add(time.Millisecond))
	backend.Close()

	client.Close()
}

// Run starts forwarding the traffic using TCP.
func (proxy *TCPProxy) Run() {
	for {
		client, err := proxy.listener.AcceptTCP()
		if err != nil {
			log.Printf("Stopping proxy on tcp/%v for tcp/%v (%s)", proxy.frontendAddr, proxy.backendAddr, err)
			return
		}
		go proxy.clientLoop(client)
	}
}

// Close stops forwarding the traffic.
func (proxy *TCPProxy) Close() { proxy.listener.Close() }

// FrontendAddr returns the TCP address on which the proxy is listening.
func (proxy *TCPProxy) FrontendAddr() net.Addr { return proxy.frontendAddr }

// BackendAddr returns the TCP proxied address.
func (proxy *TCPProxy) BackendAddr() net.Addr { return proxy.backendAddr }
