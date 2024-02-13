package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/docker/dockerversion"
	"github.com/ishidawataru/sctp"
)

func main() {
	f := os.NewFile(3, "signal-parent")
	host, container := parseFlags()

	p, err := NewProxy(host, container)
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

// parseFlags parses the flags passed on reexec to create the TCP/UDP/SCTP
// net.Addrs to map the host and container ports.
func parseFlags() (host net.Addr, container net.Addr) {
	var (
		proto         = flag.String("proto", "tcp", "proxy protocol")
		hostIP        = flag.String("host-ip", "", "host ip")
		hostPort      = flag.Int("host-port", -1, "host port")
		containerIP   = flag.String("container-ip", "", "container ip")
		containerPort = flag.Int("container-port", -1, "container port")
		printVer      = flag.Bool("v", false, "print version information and quit")
		printVersion  = flag.Bool("version", false, "print version information and quit")
	)

	flag.Parse()

	if *printVer || *printVersion {
		fmt.Printf("docker-proxy (commit %s) version %s\n", dockerversion.GitCommit, dockerversion.Version)
		os.Exit(0)
	}

	switch *proto {
	case "tcp":
		host = &net.TCPAddr{IP: net.ParseIP(*hostIP), Port: *hostPort}
		container = &net.TCPAddr{IP: net.ParseIP(*containerIP), Port: *containerPort}
	case "udp":
		host = &net.UDPAddr{IP: net.ParseIP(*hostIP), Port: *hostPort}
		container = &net.UDPAddr{IP: net.ParseIP(*containerIP), Port: *containerPort}
	case "sctp":
		host = &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP(*hostIP)}}, Port: *hostPort}
		container = &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP(*containerIP)}}, Port: *containerPort}
	default:
		log.Fatalf("unsupported protocol %s", *proto)
	}

	return host, container
}

func handleStopSignals(p Proxy) {
	s := make(chan os.Signal, 10)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)

	for range s {
		p.Close()

		os.Exit(0)
	}
}
