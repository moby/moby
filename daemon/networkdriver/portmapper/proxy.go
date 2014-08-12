package portmapper

import (
	"flag"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/docker/docker/pkg/proxy"
	"github.com/docker/docker/reexec"
)

const userlandProxyCommandName = "docker-proxy"

func init() {
	reexec.Register(userlandProxyCommandName, execProxy)
}

type UserlandProxy interface {
	Start() error
	Stop() error
}

// proxyCommand wraps an exec.Cmd to run the userland TCP and UDP
// proxies as separate processes.
type proxyCommand struct {
	cmd *exec.Cmd
}

// execProxy is the reexec function that is registered to start the userland proxies
func execProxy() {
	host, container := parseHostContainerAddrs()

	p, err := proxy.NewProxy(host, container)
	if err != nil {
		log.Fatal(err)
	}

	go handleStopSignals(p)

	// Run will block until the proxy stops
	p.Run()
}

// parseHostContainerAddrs parses the flags passed on reexec to create the TCP or UDP
// net.Addrs to map the host and container ports
func parseHostContainerAddrs() (host net.Addr, container net.Addr) {
	var (
		proto         = flag.String("proto", "tcp", "proxy protocol")
		hostIP        = flag.String("host-ip", "", "host ip")
		hostPort      = flag.Int("host-port", -1, "host port")
		containerIP   = flag.String("container-ip", "", "container ip")
		containerPort = flag.Int("container-port", -1, "container port")
	)

	flag.Parse()

	switch *proto {
	case "tcp":
		host = &net.TCPAddr{IP: net.ParseIP(*hostIP), Port: *hostPort}
		container = &net.TCPAddr{IP: net.ParseIP(*containerIP), Port: *containerPort}
	case "udp":
		host = &net.UDPAddr{IP: net.ParseIP(*hostIP), Port: *hostPort}
		container = &net.UDPAddr{IP: net.ParseIP(*containerIP), Port: *containerPort}
	default:
		log.Fatalf("unsupported protocol %s", *proto)
	}

	return host, container
}

func handleStopSignals(p proxy.Proxy) {
	s := make(chan os.Signal, 10)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM, syscall.SIGSTOP)

	for _ = range s {
		p.Close()

		os.Exit(0)
	}
}

func NewProxyCommand(proto string, hostIP net.IP, hostPort int, containerIP net.IP, containerPort int) UserlandProxy {
	args := []string{
		userlandProxyCommandName,
		"-proto", proto,
		"-host-ip", hostIP.String(),
		"-host-port", strconv.Itoa(hostPort),
		"-container-ip", containerIP.String(),
		"-container-port", strconv.Itoa(containerPort),
	}

	return &proxyCommand{
		cmd: &exec.Cmd{
			Path:   reexec.Self(),
			Args:   args,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			SysProcAttr: &syscall.SysProcAttr{
				Pdeathsig: syscall.SIGTERM, // send a sigterm to the proxy if the daemon process dies
			},
		},
	}
}

func (p *proxyCommand) Start() error {
	return p.cmd.Start()
}

func (p *proxyCommand) Stop() error {
	err := p.cmd.Process.Signal(os.Interrupt)
	p.cmd.Wait()

	return err
}
