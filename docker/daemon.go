// +build daemon

package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	apiserver "github.com/docker/docker/api/server"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/daemon"
	_ "github.com/docker/docker/daemon/execdriver/lxc"
	_ "github.com/docker/docker/daemon/execdriver/native"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/homedir"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/pidfile"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/pkg/timeutils"
	"github.com/docker/docker/pkg/ulimit"
	"github.com/docker/docker/registry"
)

type ServerCli struct {
	proto     string
	addr      string
	in        io.ReadCloser
	out       io.Writer
	err       io.Writer
	keyFile   string
	tlsConfig *tls.Config
	scheme    string
	// inFd holds file descriptor of the client's STDIN, if it's a valid file
	inFd uintptr
	// outFd holds file descriptor of the client's STDOUT, if it's a valid file
	outFd uintptr
	// isTerminalIn describes if client's STDIN is a TTY
	isTerminalIn bool
	// isTerminalOut describes if client's STDOUT is a TTY
	isTerminalOut bool
	transport     *http.Transport
}

const CanDaemon = true

var (
	daemonCfg   = daemon.Config{}
	registryCfg = &registry.Options{}
	cmd         *flag.FlagSet
)

func init() {
	registryCfg.InstallFlags()
}

func parseDaemonFlags(cmd *flag.FlagSet, args ...string) error {

	if len(args) > 0 {
		args = args[1:]
	}

	cmd.Require(flag.Exact, 0)

	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	if len(args) > 1 {
		if args[0][:1] != "-" {
			flag.Usage()
			os.Exit(0)
		}
	}

	if flag.NArg() < 1 {
		flag.Usage()
	}
	return nil
}

func installDaemonFlags() {

	srv := newServerCli(os.Stdin, os.Stdout, os.Stderr, *flTrustKey, "", "", nil)
	cmd = srv.subcmd("daemon", "[OPTIONS]", "Enable Daemon Mode", true)

	cmd.StringVar(&daemonCfg.Pidfile, []string{"p", "-pidfile"}, "/var/run/docker.pid", "Path to use for daemon PID file")
	cmd.StringVar(&daemonCfg.Root, []string{"g", "-graph"}, "/var/lib/docker", "Root of the Docker runtime")
	cmd.BoolVar(&daemonCfg.AutoRestart, []string{"#r", "#-restart"}, true, "--restart on the daemon has been deprecated in favor of --restart policies on docker run")
	cmd.BoolVar(&daemonCfg.Bridge.EnableIptables, []string{"#iptables", "-iptables"}, true, "Enable addition of iptables rules")
	cmd.BoolVar(&daemonCfg.Bridge.EnableIpForward, []string{"#ip-forward", "-ip-forward"}, true, "Enable net.ipv4.ip_forward")
	cmd.BoolVar(&daemonCfg.Bridge.EnableIpMasq, []string{"-ip-masq"}, true, "Enable IP masquerading")
	cmd.BoolVar(&daemonCfg.Bridge.EnableIPv6, []string{"-ipv6"}, false, "Enable IPv6 networking")
	cmd.StringVar(&daemonCfg.Bridge.IP, []string{"#bip", "-bip"}, "", "Specify network bridge IP")
	cmd.StringVar(&daemonCfg.Bridge.Iface, []string{"b", "-bridge"}, "", "Attach containers to a network bridge")
	cmd.StringVar(&daemonCfg.Bridge.FixedCIDR, []string{"-fixed-cidr"}, "", "IPv4 subnet for fixed IPs")
	cmd.StringVar(&daemonCfg.Bridge.FixedCIDRv6, []string{"-fixed-cidr-v6"}, "", "IPv6 subnet for fixed IPs")
	cmd.StringVar(&daemonCfg.Bridge.DefaultGatewayIPv4, []string{"-default-gateway"}, "", "Container default gateway IPv4 address")
	cmd.StringVar(&daemonCfg.Bridge.DefaultGatewayIPv6, []string{"-default-gateway-v6"}, "", "Container default gateway IPv6 address")
	cmd.BoolVar(&daemonCfg.Bridge.InterContainerCommunication, []string{"#icc", "-icc"}, true, "Enable inter-container communication")
	cmd.StringVar(&daemonCfg.GraphDriver, []string{"s", "-storage-driver"}, "", "Storage driver to use")
	cmd.StringVar(&daemonCfg.ExecDriver, []string{"e", "-exec-driver"}, "native", "Exec driver to use")
	cmd.BoolVar(&daemonCfg.EnableSelinuxSupport, []string{"-selinux-enabled"}, false, "Enable selinux support")
	cmd.IntVar(&daemonCfg.Mtu, []string{"#mtu", "-mtu"}, 0, "Set the containers network MTU")
	cmd.StringVar(&daemonCfg.SocketGroup, []string{"G", "-group"}, "docker", "Group for the unix socket")
	cmd.BoolVar(&daemonCfg.EnableCors, []string{"#api-enable-cors", "#-api-enable-cors"}, false, "Enable CORS headers in the remote API, this is deprecated by --api-cors-header")
	cmd.StringVar(&daemonCfg.CorsHeaders, []string{"-api-cors-header"}, "", "Set CORS headers in the remote API")
	opts.IPVar(cmd, &daemonCfg.Bridge.DefaultIp, []string{"#ip", "-ip"}, "0.0.0.0", "Default IP when binding container ports")
	opts.ListVar(cmd, &daemonCfg.GraphOptions, []string{"-storage-opt"}, "Set storage driver options")
	opts.ListVar(cmd, &daemonCfg.ExecOptions, []string{"-exec-opt"}, "Set exec driver options")
	// FIXME: why the inconsistency between "hosts" and "sockets"?
	opts.IPListVar(cmd, &daemonCfg.Dns, []string{"#dns", "-dns"}, "DNS server to use")
	opts.DnsSearchListVar(cmd, &daemonCfg.DnsSearch, []string{"-dns-search"}, "DNS search domains to use")
	opts.LabelListVar(cmd, &daemonCfg.Labels, []string{"-label"}, "Set key=value labels to the daemon")
	daemonCfg.Ulimits = make(map[string]*ulimit.Ulimit)
	opts.UlimitMapVar(cmd, daemonCfg.Ulimits, []string{"-default-ulimit"}, "Set default ulimits for containers")
	cmd.StringVar(&daemonCfg.LogConfig.Type, []string{"-log-driver"}, "json-file", "Default driver for container logs")

}

func (srv *ServerCli) subcmd(name, signature, description string, exitOnError bool) *flag.FlagSet {
	var errorHandling flag.ErrorHandling
	if exitOnError {
		errorHandling = flag.ExitOnError
	} else {
		errorHandling = flag.ContinueOnError
	}
	flags := flag.NewFlagSet(name, errorHandling)
	flags.Usage = func() {
		options := ""
		if flags.FlagCountUndeprecated() > 0 {
			options = "[OPTIONS] "
		}
		fmt.Fprintf(srv.out, "\nUsage: docker %s %s%s\n\n%s\n\n", name, options, signature, description)
		flags.SetOutput(srv.out)
		flags.PrintDefaults()
		os.Exit(0)
	}
	return flags
}

func newServerCli(in io.ReadCloser, out, err io.Writer, keyFile string, proto, addr string, tlsConfig *tls.Config) *ServerCli {
	var (
		inFd          uintptr
		outFd         uintptr
		isTerminalIn  = false
		isTerminalOut = false
		scheme        = "http"
	)

	if tlsConfig != nil {
		scheme = "https"
	}

	if in != nil {
		if file, ok := in.(*os.File); ok {
			inFd = file.Fd()
			isTerminalIn = term.IsTerminal(inFd)
		}
	}

	if out != nil {
		if file, ok := out.(*os.File); ok {
			outFd = file.Fd()
			isTerminalOut = term.IsTerminal(outFd)
		}
	}

	if err == nil {
		err = out
	}

	// The transport is created here for reuse during the client session
	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// Why 32? See issue 8035
	timeout := 32 * time.Second
	if proto == "unix" {
		// no need in compressing for local communications
		tr.DisableCompression = true
		tr.Dial = func(_, _ string) (net.Conn, error) {
			return net.DialTimeout(proto, addr, timeout)
		}
	} else {
		tr.Proxy = http.ProxyFromEnvironment
		tr.Dial = (&net.Dialer{Timeout: timeout}).Dial
	}

	return &ServerCli{
		proto:         proto,
		addr:          addr,
		in:            in,
		out:           out,
		err:           err,
		keyFile:       keyFile,
		inFd:          inFd,
		outFd:         outFd,
		isTerminalIn:  isTerminalIn,
		isTerminalOut: isTerminalOut,
		tlsConfig:     tlsConfig,
		scheme:        scheme,
		transport:     tr,
	}
}

func migrateKey() (err error) {
	// Migrate trust key if exists at ~/.docker/key.json and owned by current user
	oldPath := filepath.Join(homedir.Get(), ".docker", defaultTrustKeyFile)
	newPath := filepath.Join(getDaemonConfDir(), defaultTrustKeyFile)
	if _, statErr := os.Stat(newPath); os.IsNotExist(statErr) && currentUserIsOwner(oldPath) {
		defer func() {
			// Ensure old path is removed if no error occurred
			if err == nil {
				err = os.Remove(oldPath)
			} else {
				logrus.Warnf("Key migration failed, key file not removed at %s", oldPath)
			}
		}()

		if err := os.MkdirAll(getDaemonConfDir(), os.FileMode(0644)); err != nil {
			return fmt.Errorf("Unable to create daemon configuration directory: %s", err)
		}

		newFile, err := os.OpenFile(newPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("error creating key file %q: %s", newPath, err)
		}
		defer newFile.Close()

		oldFile, err := os.Open(oldPath)
		if err != nil {
			return fmt.Errorf("error opening key file %q: %s", oldPath, err)
		}
		defer oldFile.Close()

		if _, err := io.Copy(newFile, oldFile); err != nil {
			return fmt.Errorf("error copying key: %s", err)
		}

		logrus.Infof("Migrated key from %s to %s", oldPath, newPath)
	}

	return nil
}

func mainDaemon() {

	logrus.SetFormatter(&logrus.TextFormatter{TimestampFormat: timeutils.RFC3339NanoFixed})

	var pfile *pidfile.PidFile
	if daemonCfg.Pidfile != "" {
		pf, err := pidfile.New(daemonCfg.Pidfile)
		if err != nil {
			logrus.Fatalf("Error starting daemon: %v", err)
		}
		pfile = pf
		defer func() {
			if err := pfile.Remove(); err != nil {
				logrus.Error(err)
			}
		}()
	}

	if err := migrateKey(); err != nil {
		logrus.Fatal(err)
	}
	daemonCfg.TrustKeyPath = *flTrustKey

	registryService := registry.NewService(registryCfg)
	d, err := daemon.NewDaemon(&daemonCfg, registryService)
	if err != nil {
		if pfile != nil {
			if err := pfile.Remove(); err != nil {
				logrus.Error(err)
			}
		}
		logrus.Fatalf("Error starting daemon: %v", err)
	}

	logrus.Info("Daemon has completed initialization")

	logrus.WithFields(logrus.Fields{
		"version":     dockerversion.VERSION,
		"commit":      dockerversion.GITCOMMIT,
		"execdriver":  d.ExecutionDriver().Name(),
		"graphdriver": d.GraphDriver().String(),
	}).Info("Docker daemon")

	serverConfig := &apiserver.ServerConfig{
		Logging:     true,
		EnableCors:  daemonCfg.EnableCors,
		CorsHeaders: daemonCfg.CorsHeaders,
		Version:     dockerversion.VERSION,
		SocketGroup: daemonCfg.SocketGroup,
		Tls:         *flTls,
		TlsVerify:   *flTlsVerify,
		TlsCa:       *flCa,
		TlsCert:     *flCert,
		TlsKey:      *flKey,
	}

	api := apiserver.New(serverConfig)

	// The serve API routine never exits unless an error occurs
	// We need to start it as a goroutine and wait on it so
	// daemon doesn't exit
	serveAPIWait := make(chan error)
	go func() {
		if err := api.ServeApi(flHosts); err != nil {
			logrus.Errorf("ServeAPI error: %v", err)
			serveAPIWait <- err
			return
		}
		serveAPIWait <- nil
	}()

	signal.Trap(func() {
		api.Close()
		<-serveAPIWait
		shutdownDaemon(d, 15)
		if pfile != nil {
			if err := pfile.Remove(); err != nil {
				logrus.Error(err)
			}
		}
	})

	// after the daemon is done setting up we can tell the api to start
	// accepting connections with specified daemon
	api.AcceptConnections(d)

	// Daemon is fully initialized and handling API traffic
	// Wait for serve API to complete
	errAPI := <-serveAPIWait
	shutdownDaemon(d, 15)
	if errAPI != nil {
		if pfile != nil {
			if err := pfile.Remove(); err != nil {
				logrus.Error(err)
			}
		}
		logrus.Fatalf("Shutting down due to ServeAPI error: %v", errAPI)
	}
}

// shutdownDaemon just wraps daemon.Shutdown() to handle a timeout in case
// d.Shutdown() is waiting too long to kill container or worst it's
// blocked there
func shutdownDaemon(d *daemon.Daemon, timeout time.Duration) {
	ch := make(chan struct{})
	go func() {
		d.Shutdown()
		close(ch)
	}()
	select {
	case <-ch:
		logrus.Debug("Clean shutdown succeded")
	case <-time.After(timeout * time.Second):
		logrus.Error("Force shutdown daemon")
	}
}

// currentUserIsOwner checks whether the current user is the owner of the given
// file.
func currentUserIsOwner(f string) bool {
	if fileInfo, err := system.Stat(f); err == nil && fileInfo != nil {
		if int(fileInfo.Uid()) == os.Getuid() {
			return true
		}
	}
	return false
}
