package portmapper

import (
	"flag"
	"fmt"
	"io"
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
	"github.com/docker/docker/pkg/reexec"
)

const userlandProxyCommandName = "docker-proxy"

func init() {
	reexec.Register(userlandProxyCommandName, execProxy)
}

type userlandProxy interface {
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
	f := os.NewFile(3, "signal-parent")
	host, container := parseHostContainerAddrs()

	p, err := proxy.NewProxy(host, container)
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

func newProxyCommand(proto string, hostIP net.IP, hostPort int, containerIP net.IP, containerPort int) userlandProxy {
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
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("proxy unable to open os.Pipe %s", err)
	}
	defer r.Close()
	p.cmd.ExtraFiles = []*os.File{w}
	if err := p.cmd.Start(); err != nil {
		return err
	}
	w.Close()

	errchan := make(chan error, 1)
	go func() {
		buf := make([]byte, 2)
		r.Read(buf)

		if string(buf) != "0\n" {
			errStr, err := ioutil.ReadAll(r)
			if err != nil {
				errchan <- fmt.Errorf("Error reading exit status from userland proxy: %v", err)
				return
			}

			errchan <- fmt.Errorf("Error starting userland proxy: %s", errStr)
			return
		}
		errchan <- nil
	}()

	select {
	case err := <-errchan:
		return err
	case <-time.After(16 * time.Second):
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

// dummyProxy just listen on some port, it is needed to prevent accidental
// port allocations on bound port, because without userland proxy we using
// iptables rules and not net.Listen
type dummyProxy struct {
	listener io.Closer
	addr     net.Addr
}

func newDummyProxy(proto string, hostIP net.IP, hostPort int) userlandProxy {
	switch proto {
	case "tcp":
		addr := &net.TCPAddr{IP: hostIP, Port: hostPort}
		return &dummyProxy{addr: addr}
	case "udp":
		addr := &net.UDPAddr{IP: hostIP, Port: hostPort}
		return &dummyProxy{addr: addr}
	}
	return nil
}

func (p *dummyProxy) Start() error {
	switch addr := p.addr.(type) {
	case *net.TCPAddr:
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return err
		}
		p.listener = l
	case *net.UDPAddr:
		l, err := net.ListenUDP("udp", addr)
		if err != nil {
			return err
		}
		p.listener = l
	default:
		return fmt.Errorf("Unknown addr type: %T", p.addr)
	}
	return nil
}

func (p *dummyProxy) Stop() error {
	if p.listener != nil {
		return p.listener.Close()
	}
	return nil
}
