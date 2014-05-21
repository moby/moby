package daemon

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/dockerversion"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/opts"
	flag "github.com/dotcloud/docker/pkg/mflag"
	"github.com/dotcloud/docker/utils"
)

const (
	defaultCaFile   = "ca.pem"
	defaultKeyFile  = "key.pem"
	defaultCertFile = "cert.pem"
)

var (
	dockerConfDir = os.Getenv("HOME") + "/.docker/"
)

func DaemonInit(job *engine.Job) engine.Status {
	flags := flag.NewFlagSet("daemon-init", flag.ExitOnError)

	var (
		flDaemon             = flags.Bool([]string{"d", "-daemon"}, false, "Enable daemon mode")
		flAutoRestart        = flags.Bool([]string{"r", "-restart"}, true, "Restart previously running containers")
		bridgeName           = flags.String([]string{"b", "-bridge"}, "", "Attach containers to a pre-existing network bridge\nuse 'none' to disable container networking")
		bridgeIp             = flags.String([]string{"#bip", "-bip"}, "", "Use this CIDR notation address for the network bridge's IP, not compatible with -b")
		pidfile              = flags.String([]string{"p", "-pidfile"}, "/var/run/docker.pid", "Path to use for daemon PID file")
		flRoot               = flags.String([]string{"g", "-graph"}, "/var/lib/docker", "Path to use as the root of the docker runtime")
		flSocketGroup        = flags.String([]string{"G", "-group"}, "docker", "Group to assign the unix socket specified by -H when running in daemon mode\nuse '' (the empty string) to disable setting of a group")
		flEnableCors         = flags.Bool([]string{"#api-enable-cors", "-api-enable-cors"}, false, "Enable CORS headers in the remote API")
		flDns                = opts.NewListOpts(opts.ValidateIp4Address)
		flDnsSearch          = opts.NewListOpts(opts.ValidateDomain)
		flEnableIptables     = flags.Bool([]string{"#iptables", "-iptables"}, true, "Enable Docker's addition of iptables rules")
		flEnableIpForward    = flags.Bool([]string{"#ip-forward", "-ip-forward"}, true, "Enable net.ipv4.ip_forward")
		flDefaultIp          = flags.String([]string{"#ip", "-ip"}, "0.0.0.0", "Default IP address to use when binding container ports")
		flInterContainerComm = flags.Bool([]string{"#icc", "-icc"}, true, "Enable inter-container communication")
		flGraphDriver        = flags.String([]string{"s", "-storage-driver"}, "", "Force the docker runtime to use a specific storage driver")
		flExecDriver         = flags.String([]string{"e", "-exec-driver"}, "native", "Force the docker runtime to use a specific exec driver")
		flHosts              = opts.NewListOpts(api.ValidateHost)
		flMtu                = flags.Int([]string{"#mtu", "-mtu"}, 0, "Set the containers network MTU\nif no value is provided: default to the default route MTU or 1500 if no default route is available")
		flTls                = flags.Bool([]string{"-tls"}, false, "Use TLS; implied by tls-verify flags")
		flTlsVerify          = flags.Bool([]string{"-tlsverify"}, false, "Use TLS and verify the remote (daemon: verify client, client: verify daemon)")
		flCa                 = flags.String([]string{"-tlscacert"}, dockerConfDir+defaultCaFile, "Trust only remotes providing a certificate signed by the CA given here")
		flCert               = flags.String([]string{"-tlscert"}, dockerConfDir+defaultCertFile, "Path to TLS certificate file")
		flKey                = flags.String([]string{"-tlskey"}, dockerConfDir+defaultKeyFile, "Path to TLS key file")
		flSelinuxEnabled     = flags.Bool([]string{"-selinux-enabled"}, false, "Enable selinux support")
	)

	flags.Bool([]string{"D", "-debug"}, false, "Enable debug mode")

	flags.Var(&flDns, []string{"#dns", "-dns"}, "Force docker to use specific DNS servers")
	flags.Var(&flDnsSearch, []string{"-dns-search"}, "Force Docker to use specific DNS search domains")
	flags.Var(&flHosts, []string{"H", "-host"}, "The socket(s) to bind to in daemon mode\nspecified using one or more tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.")

	flags.Parse(job.Args)

	if flHosts.Len() == 0 {
		defaultHost := os.Getenv("DOCKER_HOST")

		if defaultHost == "" || *flDaemon {
			// If we do not have a host, default to unix socket
			defaultHost = fmt.Sprintf("unix://%s", api.DEFAULTUNIXSOCKET)
		}
		if _, err := api.ValidateHost(defaultHost); err != nil {
			log.Fatal(err)
		}
		flHosts.Set(defaultHost)
	}

	if *bridgeName != "" && *bridgeIp != "" {
		log.Fatal("You specified -b & --bip, mutually exclusive options. Please specify only one.")
	}

	if runtime.GOOS != "linux" {
		log.Fatalf("The Docker daemon is only supported on linux")
	}
	if os.Geteuid() != 0 {
		log.Fatalf("The Docker daemon needs to be run as root")
	}

	if flags.NArg() != 0 {
		flags.Usage()
		return engine.StatusErr
	}

	// set up the TempDir to use a canonical path
	tmp := os.TempDir()
	realTmp, err := utils.ReadSymlinkedDirectory(tmp)
	if err != nil {
		log.Fatalf("Unable to get the full path to the TempDir (%s): %s", tmp, err)
	}
	os.Setenv("TMPDIR", realTmp)

	// get the canonical path to the Docker root directory
	root := *flRoot
	var realRoot string
	if _, err := os.Stat(root); err != nil && os.IsNotExist(err) {
		realRoot = root
	} else {
		realRoot, err = utils.ReadSymlinkedDirectory(root)
		if err != nil {
			log.Fatalf("Unable to get the full path to root (%s): %s", root, err)
		}
	}
	if err := checkKernelAndArch(); err != nil {
		log.Fatal(err)
	}

	// load the daemon in the background so we can immediately start
	// the http api so that connections don't fail while the daemon
	// is booting
	go func() {
		// Load plugin: httpapi
		initJob := job.Job("initserver")
		initJob.Setenv("Pidfile", *pidfile)
		initJob.Setenv("Root", realRoot)
		initJob.SetenvBool("AutoRestart", *flAutoRestart)
		initJob.SetenvList("Dns", flDns.GetAll())
		initJob.SetenvList("DnsSearch", flDnsSearch.GetAll())
		initJob.SetenvBool("EnableIptables", *flEnableIptables)
		initJob.SetenvBool("EnableIpForward", *flEnableIpForward)
		initJob.Setenv("BridgeIface", *bridgeName)
		initJob.Setenv("BridgeIP", *bridgeIp)
		initJob.Setenv("DefaultIp", *flDefaultIp)
		initJob.SetenvBool("InterContainerCommunication", *flInterContainerComm)
		initJob.Setenv("GraphDriver", *flGraphDriver)
		initJob.Setenv("ExecDriver", *flExecDriver)
		initJob.SetenvInt("Mtu", *flMtu)
		initJob.SetenvBool("EnableSelinuxSupport", *flSelinuxEnabled)

		if err := initJob.Run(); err != nil {
			log.Fatal(err)
		}

		// after the daemon is done setting up we can tell the api to start
		// accepting connections
		if err := job.Job("acceptconnections").Run(); err != nil {
			log.Fatal(err)
		}
	}()

	// TODO actually have a resolved graphdriver to show?
	log.Printf("docker daemon: %s %s; execdriver: %s; graphdriver: %s",
		dockerversion.VERSION,
		dockerversion.GITCOMMIT,
		*flExecDriver,
		*flGraphDriver)

	// Serve api
	serveapiJob := job.Job("serveapi", flHosts.GetAll()...)
	serveapiJob.SetenvBool("Logging", true)
	serveapiJob.SetenvBool("EnableCors", *flEnableCors)
	serveapiJob.Setenv("Version", dockerversion.VERSION)
	serveapiJob.Setenv("SocketGroup", *flSocketGroup)

	serveapiJob.SetenvBool("Tls", *flTls)
	serveapiJob.SetenvBool("TlsVerify", *flTlsVerify)
	serveapiJob.Setenv("TlsCa", *flCa)
	serveapiJob.Setenv("TlsCert", *flCert)
	serveapiJob.Setenv("TlsKey", *flKey)
	serveapiJob.SetenvBool("BufferRequests", true)

	if err := serveapiJob.Run(); err != nil {
		log.Fatal(err)
	}

	return engine.StatusOK
}

func checkKernelAndArch() error {
	// Check for unsupported architectures
	if runtime.GOARCH != "amd64" {
		return fmt.Errorf("The docker runtime currently only supports amd64 (not %s). This will change in the future. Aborting.", runtime.GOARCH)
	}
	// Check for unsupported kernel versions
	// FIXME: it would be cleaner to not test for specific versions, but rather
	// test for specific functionalities.
	// Unfortunately we can't test for the feature "does not cause a kernel panic"
	// without actually causing a kernel panic, so we need this workaround until
	// the circumstances of pre-3.8 crashes are clearer.
	// For details see http://github.com/dotcloud/docker/issues/407
	if k, err := utils.GetKernelVersion(); err != nil {
		log.Printf("WARNING: %s\n", err)
	} else {
		if utils.CompareKernelVersion(k, &utils.KernelVersionInfo{Kernel: 3, Major: 8, Minor: 0}) < 0 {
			if os.Getenv("DOCKER_NOWARN_KERNEL_VERSION") == "" {
				log.Printf("WARNING: You are running linux kernel version %s, which might be unstable running docker. Please upgrade your kernel to 3.8.0.", k.String())
			}
		}
	}
	return nil
}
