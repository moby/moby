package proxy

import (
	"log"
	"net"
	"sync"
)

type TCPProxy struct {
	listener     *net.TCPListener
	frontendAddr *net.TCPAddr
	backendAddr  *net.TCPAddr
	quit         chan struct{}
	mu           sync.Mutex // protects closed field
	closed       bool
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
		quit:         make(chan struct{}),
	}, nil
}

func (proxy *TCPProxy) Run() {
	for {
		client, err := proxy.listener.Accept()
		if err != nil {
			log.Printf("Stopping proxy on tcp/%v for tcp/%v (err: %s)", proxy.frontendAddr, proxy.backendAddr, err)
			return
		}
		go func(client net.Conn) {
			defer client.Close()

			backend, err := net.DialTCP("tcp", nil, proxy.backendAddr)
			if err != nil {
				log.Printf("Can't forward traffic to backend tcp/%v: %s\n", proxy.backendAddr, err)
				client.Close()
				return
			}
			defer backend.Close()

			c1 := goTransfert(backend, client)
			defer close(c1)
			c2 := goTransfert(client, backend)
			defer close(c2)

			select {
			case <-proxy.quit:
				return
			case <-c1:
			case <-c2:
			}
			select {
			case <-proxy.quit:
				return
			case <-c1:
			case <-c2:
			}

		}(client)
	}
}

func (proxy *TCPProxy) Close() error {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()

	if !proxy.closed {
		close(proxy.quit)
		proxy.closed = true
		proxy.listener.Close()
	}
	return nil
}
func (proxy *TCPProxy) FrontendAddr() net.Addr { return proxy.frontendAddr }
func (proxy *TCPProxy) BackendAddr() net.Addr  { return proxy.backendAddr }
