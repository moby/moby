package portmapper

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

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
		os.Stdout.WriteString("1\n")
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	go handleStopSignals(p)
	os.Stdout.WriteString("0\n")

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
			Path: reexec.Self(),
			Args: args,
			SysProcAttr: &syscall.SysProcAttr{
				Pdeathsig: syscall.SIGTERM, // send a sigterm to the proxy if the daemon process dies
			},
		},
	}
}

func (p *proxyCommand) Start() error {
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdout.Close()
	stderr, err := p.cmd.StderrPipe()
	if err != nil {
		return err
	}
	defer stderr.Close()
	if err := p.cmd.Start(); err != nil {
		return err
	}

	errchan := make(chan error, 1)
	go func() {
		buf := make([]byte, 2)
		stdout.Read(buf)

		if string(buf) != "0\n" {
			errStr, _ := ioutil.ReadAll(stderr)
			errchan <- fmt.Errorf("Error starting userland proxy: %s", errStr)
			return
		}
		errchan <- nil
	}()

	select {
	case err := <-errchan:
		return err
	case <-time.After(1 * time.Second):
		return fmt.Errorf("Timed out proxy starting the userland proxy")
	}
}

func (p *proxyCommand) Stop() error {
	if p.cmd.Process != nil {
		if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
			return err
		}
		return p.cmd.Wait()
	}
	return nil
}
