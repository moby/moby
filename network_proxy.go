package docker

import (
	"encoding/binary"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"log"
	"net"
	"sync"
	"syscall"
	"time"
)

const (
	UDPConnTrackTimeout = 90 * time.Second
	UDPBufSize          = 2048
)

type Proxy interface {
	// Start forwarding traffic back and forth the front and back-end
	// addresses.
	Run()
	// Stop forwarding traffic and close both ends of the Proxy.
	Close()
	// Return the address on which the proxy is listening.
	FrontendAddr() net.Addr
	// Return the proxied address.
	BackendAddr() net.Addr
}

type TCPProxy struct {
	listener     *net.TCPListener
	frontendAddr *net.TCPAddr
	backendAddr  *net.TCPAddr
}

func NewTCPProxy(frontendAddr, backendAddr *net.TCPAddr) (*TCPProxy, error) {
	listener, err := net.ListenTCP("tcp", frontendAddr)
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

func (proxy *TCPProxy) clientLoop(client *net.TCPConn, quit chan bool) {
	backend, err := net.DialTCP("tcp", nil, proxy.backendAddr)
	if err != nil {
		log.Printf("Can't forward traffic to backend tcp/%v: %v\n", proxy.backendAddr, err.Error())
		client.Close()
		return
	}

	event := make(chan int64)
	var broker = func(to, from *net.TCPConn) {
		written, err := io.Copy(to, from)
		if err != nil {
			err, ok := err.(*net.OpError)
			// If the socket we are writing to is shutdown with
			// SHUT_WR, forward it to the other end of the pipe:
			if ok && err.Err == syscall.EPIPE {
				from.CloseWrite()
			}
		}
		event <- written
	}
	utils.Debugf("Forwarding traffic between tcp/%v and tcp/%v", client.RemoteAddr(), backend.RemoteAddr())
	go broker(client, backend)
	go broker(backend, client)

	var transferred int64 = 0
	for i := 0; i < 2; i++ {
		select {
		case written := <-event:
			transferred += written
		case <-quit:
			// Interrupt the two brokers and "join" them.
			client.Close()
			backend.Close()
			for ; i < 2; i++ {
				transferred += <-event
			}
			goto done
		}
	}
	client.Close()
	backend.Close()
done:
	utils.Debugf("%v bytes transferred between tcp/%v and tcp/%v", transferred, client.RemoteAddr(), backend.RemoteAddr())
}

func (proxy *TCPProxy) Run() {
	quit := make(chan bool)
	defer close(quit)
	utils.Debugf("Starting proxy on tcp/%v for tcp/%v", proxy.frontendAddr, proxy.backendAddr)
	for {
		client, err := proxy.listener.Accept()
		if err != nil {
			utils.Debugf("Stopping proxy on tcp/%v for tcp/%v (%v)", proxy.frontendAddr, proxy.backendAddr, err.Error())
			return
		}
		go proxy.clientLoop(client.(*net.TCPConn), quit)
	}
}

func (proxy *TCPProxy) Close()                 { proxy.listener.Close() }
func (proxy *TCPProxy) FrontendAddr() net.Addr { return proxy.frontendAddr }
func (proxy *TCPProxy) BackendAddr() net.Addr  { return proxy.backendAddr }

// A net.Addr where the IP is split into two fields so you can use it as a key
// in a map:
type connTrackKey struct {
	IPHigh uint64
	IPLow  uint64
	Port   int
}

func newConnTrackKey(addr *net.UDPAddr) *connTrackKey {
	if len(addr.IP) == net.IPv4len {
		return &connTrackKey{
			IPHigh: 0,
			IPLow:  uint64(binary.BigEndian.Uint32(addr.IP)),
			Port:   addr.Port,
		}
	}
	return &connTrackKey{
		IPHigh: binary.BigEndian.Uint64(addr.IP[:8]),
		IPLow:  binary.BigEndian.Uint64(addr.IP[8:]),
		Port:   addr.Port,
	}
}

type connTrackMap map[connTrackKey]*net.UDPConn

type UDPProxy struct {
	listener       *net.UDPConn
	frontendAddr   *net.UDPAddr
	backendAddr    *net.UDPAddr
	connTrackTable connTrackMap
	connTrackLock  sync.Mutex
}

func NewUDPProxy(frontendAddr, backendAddr *net.UDPAddr) (*UDPProxy, error) {
	listener, err := net.ListenUDP("udp", frontendAddr)
	if err != nil {
		return nil, err
	}
	return &UDPProxy{
		listener:       listener,
		frontendAddr:   listener.LocalAddr().(*net.UDPAddr),
		backendAddr:    backendAddr,
		connTrackTable: make(connTrackMap),
	}, nil
}

func (proxy *UDPProxy) replyLoop(proxyConn *net.UDPConn, clientAddr *net.UDPAddr, clientKey *connTrackKey) {
	defer func() {
		proxy.connTrackLock.Lock()
		delete(proxy.connTrackTable, *clientKey)
		proxy.connTrackLock.Unlock()
		utils.Debugf("Done proxying between udp/%v and udp/%v", clientAddr.String(), proxy.backendAddr.String())
		proxyConn.Close()
	}()

	readBuf := make([]byte, UDPBufSize)
	for {
		proxyConn.SetReadDeadline(time.Now().Add(UDPConnTrackTimeout))
	again:
		read, err := proxyConn.Read(readBuf)
		if err != nil {
			if err, ok := err.(*net.OpError); ok && err.Err == syscall.ECONNREFUSED {
				// This will happen if the last write failed
				// (e.g: nothing is actually listening on the
				// proxied port on the container), ignore it
				// and continue until UDPConnTrackTimeout
				// expires:
				goto again
			}
			return
		}
		for i := 0; i != read; {
			written, err := proxy.listener.WriteToUDP(readBuf[i:read], clientAddr)
			if err != nil {
				return
			}
			i += written
			utils.Debugf("Forwarded %v/%v bytes to udp/%v", i, read, clientAddr.String())
		}
	}
}

func (proxy *UDPProxy) Run() {
	readBuf := make([]byte, UDPBufSize)
	utils.Debugf("Starting proxy on udp/%v for udp/%v", proxy.frontendAddr, proxy.backendAddr)
	for {
		read, from, err := proxy.listener.ReadFromUDP(readBuf)
		if err != nil {
			// NOTE: Apparently ReadFrom doesn't return
			// ECONNREFUSED like Read do (see comment in
			// UDPProxy.replyLoop)
			utils.Debugf("Stopping proxy on udp/%v for udp/%v (%v)", proxy.frontendAddr, proxy.backendAddr, err.Error())
			break
		}

		fromKey := newConnTrackKey(from)
		proxy.connTrackLock.Lock()
		proxyConn, hit := proxy.connTrackTable[*fromKey]
		if !hit {
			proxyConn, err = net.DialUDP("udp", nil, proxy.backendAddr)
			if err != nil {
				log.Printf("Can't proxy a datagram to udp/%s: %v\n", proxy.backendAddr.String(), err)
				continue
			}
			proxy.connTrackTable[*fromKey] = proxyConn
			go proxy.replyLoop(proxyConn, from, fromKey)
		}
		proxy.connTrackLock.Unlock()
		for i := 0; i != read; {
			written, err := proxyConn.Write(readBuf[i:read])
			if err != nil {
				log.Printf("Can't proxy a datagram to udp/%s: %v\n", proxy.backendAddr.String(), err)
				break
			}
			i += written
			utils.Debugf("Forwarded %v/%v bytes to udp/%v", i, read, proxy.backendAddr.String())
		}
	}
}

func (proxy *UDPProxy) Close() {
	proxy.listener.Close()
	proxy.connTrackLock.Lock()
	defer proxy.connTrackLock.Unlock()
	for _, conn := range proxy.connTrackTable {
		conn.Close()
	}
}

func (proxy *UDPProxy) FrontendAddr() net.Addr { return proxy.frontendAddr }
func (proxy *UDPProxy) BackendAddr() net.Addr  { return proxy.backendAddr }

func NewProxy(frontendAddr, backendAddr net.Addr) (Proxy, error) {
	switch frontendAddr.(type) {
	case *net.UDPAddr:
		return NewUDPProxy(frontendAddr.(*net.UDPAddr), backendAddr.(*net.UDPAddr))
	case *net.TCPAddr:
		return NewTCPProxy(frontendAddr.(*net.TCPAddr), backendAddr.(*net.TCPAddr))
	default:
		panic(fmt.Errorf("Unsupported protocol"))
	}
}
