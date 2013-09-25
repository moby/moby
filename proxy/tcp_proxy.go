package proxy

import (
	"github.com/dotcloud/docker/utils"
	"io"
	"log"
	"net"
	"syscall"
)

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
		to.CloseRead()
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
