package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/docker/dockerversion"
	"github.com/ishidawataru/sctp"
)

// The caller is expected to pass-in open file descriptors ...
const (
	// Pipe for reporting status, as a string. "0\n" if the proxy
	// started normally. "1\n<error message>" otherwise.
	parentPipeFd uintptr = 3 + iota
	// If -use-listen-fd=true, a listening socket ready to accept TCP
	// connections or receive UDP. (Without that option on the command
	// line, the listener needs to be opened by docker-proxy, for
	// compatibility with older docker daemons. In this case fd 4
	// may belong to the Go runtime.)
	listenSockFd
)

func main() {
	// Mark any files we expect to inherit as close-on-exec
	// so that they are not unexpectedly inherited by any child processes
	// if we ever need docker-proxy to exec something.
	// This is safe to do even if the fd belongs to the Go runtime
	// as it would be a no-op:
	// the Go runtime marks all file descriptors it opens as close-on-exec.
	// See the godoc for syscall.ForkLock for more information.
	syscall.CloseOnExec(int(parentPipeFd))
	syscall.CloseOnExec(int(listenSockFd))

	config := parseFlags()
	p, err := newProxy(config)
	if config.ListenSock != nil {
		config.ListenSock.Close()
	}

	_ = syscall.SetNonblock(int(parentPipeFd), true)
	f := os.NewFile(parentPipeFd, "signal-parent")
	if err != nil {
		fmt.Fprintf(f, "1\n%s", err)
		f.Close()
		os.Exit(1)
	}
	go handleStopSignals(p)
	fmt.Fprint(f, "0\n")
	f.Close()

	// Run will block until the proxy stops
	p.Run()
}

func newProxy(config ProxyConfig) (p Proxy, err error) {
	ipv := ipv4
	if config.HostIP.To4() == nil {
		ipv = ipv6
	}

	switch config.Proto {
	case "tcp":
		var listener *net.TCPListener
		if config.ListenSock == nil {
			// Fall back to HostIP:HostPort if no socket on fd 4, for compatibility with older daemons.
			hostAddr := &net.TCPAddr{IP: config.HostIP, Port: config.HostPort}
			listener, err = net.ListenTCP("tcp"+string(ipv), hostAddr)
			if err != nil {
				return nil, fmt.Errorf("failed to listen on %s: %w", hostAddr, err)
			}
		} else {
			l, err := net.FileListener(config.ListenSock)
			if err != nil {
				return nil, err
			}
			var ok bool
			listener, ok = l.(*net.TCPListener)
			if !ok {
				return nil, fmt.Errorf("unexpected socket type for listener fd: %s", l.Addr().Network())
			}
		}
		container := &net.TCPAddr{IP: config.ContainerIP, Port: config.ContainerPort}
		p, err = NewTCPProxy(listener, container)
	case "udp":
		var listener *net.UDPConn
		if config.ListenSock == nil {
			// Fall back to HostIP:HostPort if no socket on fd 4, for compatibility with older daemons.
			hostAddr := &net.UDPAddr{IP: config.HostIP, Port: config.HostPort}
			listener, err = net.ListenUDP("udp"+string(ipv), hostAddr)
			if err != nil {
				return nil, fmt.Errorf("failed to listen on %s: %w", hostAddr, err)
			}
		} else {
			l, err := net.FilePacketConn(config.ListenSock)
			if err != nil {
				return nil, err
			}
			var ok bool
			listener, ok = l.(*net.UDPConn)
			if !ok {
				return nil, fmt.Errorf("unexpected socket type for listener fd: %s", l.LocalAddr().Network())
			}
		}
		container := &net.UDPAddr{IP: config.ContainerIP, Port: config.ContainerPort}
		p, err = NewUDPProxy(listener, container)
	case "sctp":
		var listener *sctp.SCTPListener
		if config.ListenSock != nil {
			// There's no way to construct an SCTPListener from a file descriptor at the moment.
			// If a socket has been passed in, it's probably from a newer daemon using a version
			// of the sctp module that does allow it.
			return nil, errors.New("cannot use supplied SCTP socket, check the latest docker-proxy is in your $PATH")
		}
		hostAddr := &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: config.HostIP}}, Port: config.HostPort}
		container := &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: config.ContainerIP}}, Port: config.ContainerPort}
		listener, err = sctp.ListenSCTP("sctp"+string(ipv), hostAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to listen on %s: %w", hostAddr, err)
		}
		p, err = NewSCTPProxy(listener, container)
	default:
		return nil, fmt.Errorf("unsupported protocol %s", config.Proto)
	}

	return p, err
}

type ProxyConfig struct {
	Proto                   string
	HostIP, ContainerIP     net.IP
	HostPort, ContainerPort int
	ListenSock              *os.File
}

// parseFlags parses the flags passed on reexec to create the TCP/UDP/SCTP
// net.Addrs to map the host and container ports.
func parseFlags() ProxyConfig {
	var (
		config      ProxyConfig
		useListenFd bool
		printVer    bool
	)
	flag.StringVar(&config.Proto, "proto", "tcp", "proxy protocol")
	flag.TextVar(&config.HostIP, "host-ip", net.IPv4zero, "host ip")
	flag.IntVar(&config.HostPort, "host-port", -1, "host port")
	flag.TextVar(&config.ContainerIP, "container-ip", net.IPv4zero, "container ip")
	flag.IntVar(&config.ContainerPort, "container-port", -1, "container port")
	flag.BoolVar(&useListenFd, "use-listen-fd", false, "use a supplied listen fd")
	flag.BoolVar(&printVer, "v", false, "print version information and quit")
	flag.BoolVar(&printVer, "version", false, "print version information and quit")
	flag.Parse()

	if printVer {
		fmt.Printf("docker-proxy (commit %s) version %s\n", dockerversion.GitCommit, dockerversion.Version)
		os.Exit(0)
	}

	if useListenFd {
		_ = syscall.SetNonblock(int(listenSockFd), true)
		config.ListenSock = os.NewFile(listenSockFd, "listen-sock")
	}

	return config
}

func handleStopSignals(p Proxy) {
	s := make(chan os.Signal, 10)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)

	for range s {
		p.Close()

		os.Exit(0)
	}
}
