package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
<<<<<<< HEAD
	"path"
||||||| parent of af6ac83... Use docker daemon command instead of -d
	"path/filepath"
	"runtime"
=======
	"path/filepath"
	"strconv"
>>>>>>> af6ac83... Use docker daemon command instead of -d
	"strings"

<<<<<<< HEAD
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/dockerversion"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/reexec"
	"github.com/docker/docker/utils"
	"github.com/docker/libtrust"
||||||| parent of af6ac83... Use docker daemon command instead of -d
	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/api/client"
	"github.com/dotcloud/docker/builtins"
	"github.com/dotcloud/docker/dockerversion"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/opts"
	flag "github.com/dotcloud/docker/pkg/mflag"
	"github.com/dotcloud/docker/sysinit"
	"github.com/dotcloud/docker/utils"
=======
	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/api/client"
	"github.com/dotcloud/docker/dockerversion"
	"github.com/dotcloud/docker/opts"
	flag "github.com/dotcloud/docker/pkg/mflag"
	"github.com/dotcloud/docker/sysinit"
	"github.com/dotcloud/docker/utils"
>>>>>>> af6ac83... Use docker daemon command instead of -d
)

<<<<<<< HEAD
const (
	defaultTrustKeyFile = "key.json"
	defaultCaFile       = "ca.pem"
	defaultKeyFile      = "key.pem"
	defaultCertFile     = "cert.pem"
)

||||||| parent of af6ac83... Use docker daemon command instead of -d
const (
	defaultCaFile   = "ca.pem"
	defaultKeyFile  = "key.pem"
	defaultCertFile = "cert.pem"
)

var (
	dockerConfDir = os.Getenv("DOCKER_CONFIG")
)

=======
>>>>>>> af6ac83... Use docker daemon command instead of -d
func main() {
<<<<<<< HEAD
	if reexec.Init() {
||||||| parent of af6ac83... Use docker daemon command instead of -d
	if len(dockerConfDir) == 0 {
		dockerConfDir = filepath.Join(os.Getenv("HOME"), ".docker")
	}
	if selfPath := utils.SelfPath(); strings.Contains(selfPath, ".dockerinit") {
		// Running in init mode
		sysinit.SysInit()
=======
	for j := 1; j < len(os.Args); j++ {
		if os.Args[j] == "-d" || os.Args[j] == "--daemon" {
			log.Println("-d is deprecated. Please use daemon command")
			os.Args[j] = "daemon"
		}
	}

	if len(client.DockerConfDir) == 0 {
		client.DockerConfDir = filepath.Join(os.Getenv("HOME"), ".docker")
	}

	if selfPath := utils.SelfPath(); strings.Contains(selfPath, ".dockerinit") {
		// Running in init mode
		sysinit.SysInit()
>>>>>>> af6ac83... Use docker daemon command instead of -d
		return
	}
<<<<<<< HEAD
||||||| parent of af6ac83... Use docker daemon command instead of -d

	var (
		flVersion            = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
		flDaemon             = flag.Bool([]string{"d", "-daemon"}, false, "Enable daemon mode")
		flGraphOpts          opts.ListOpts
		flDebug              = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
		flAutoRestart        = flag.Bool([]string{"r", "-restart"}, true, "Restart previously running containers")
		bridgeName           = flag.String([]string{"b", "-bridge"}, "", "Attach containers to a pre-existing network bridge\nuse 'none' to disable container networking")
		bridgeIp             = flag.String([]string{"#bip", "-bip"}, "", "Use this CIDR notation address for the network bridge's IP, not compatible with -b")
		pidfile              = flag.String([]string{"p", "-pidfile"}, "/var/run/docker.pid", "Path to use for daemon PID file")
		flRoot               = flag.String([]string{"g", "-graph"}, "/var/lib/docker", "Path to use as the root of the Docker runtime")
		flSocketGroup        = flag.String([]string{"G", "-group"}, "docker", "Group to assign the unix socket specified by -H when running in daemon mode\nuse '' (the empty string) to disable setting of a group")
		flEnableCors         = flag.Bool([]string{"#api-enable-cors", "-api-enable-cors"}, false, "Enable CORS headers in the remote API")
		flDns                = opts.NewListOpts(opts.ValidateIPAddress)
		flDnsSearch          = opts.NewListOpts(opts.ValidateDnsSearch)
		flEnableIptables     = flag.Bool([]string{"#iptables", "-iptables"}, true, "Enable Docker's addition of iptables rules")
		flEnableIpForward    = flag.Bool([]string{"#ip-forward", "-ip-forward"}, true, "Enable net.ipv4.ip_forward")
		flDefaultIp          = flag.String([]string{"#ip", "-ip"}, "0.0.0.0", "Default IP address to use when binding container ports")
		flInterContainerComm = flag.Bool([]string{"#icc", "-icc"}, true, "Enable inter-container communication")
		flGraphDriver        = flag.String([]string{"s", "-storage-driver"}, "", "Force the Docker runtime to use a specific storage driver")
		flExecDriver         = flag.String([]string{"e", "-exec-driver"}, "native", "Force the Docker runtime to use a specific exec driver")
		flHosts              = opts.NewListOpts(api.ValidateHost)
		flMtu                = flag.Int([]string{"#mtu", "-mtu"}, 0, "Set the containers network MTU\nif no value is provided: default to the default route MTU or 1500 if no default route is available")
		flTls                = flag.Bool([]string{"-tls"}, false, "Use TLS; implied by tls-verify flags")
		flTlsVerify          = flag.Bool([]string{"-tlsverify"}, false, "Use TLS and verify the remote (daemon: verify client, client: verify daemon)")
		flCa                 = flag.String([]string{"-tlscacert"}, filepath.Join(dockerConfDir, defaultCaFile), "Trust only remotes providing a certificate signed by the CA given here")
		flCert               = flag.String([]string{"-tlscert"}, filepath.Join(dockerConfDir, defaultCertFile), "Path to TLS certificate file")
		flKey                = flag.String([]string{"-tlskey"}, filepath.Join(dockerConfDir, defaultKeyFile), "Path to TLS key file")
		flSelinuxEnabled     = flag.Bool([]string{"-selinux-enabled"}, false, "Enable selinux support. SELinux does not presently support the BTRFS storage driver")
	)
	flag.Var(&flDns, []string{"#dns", "-dns"}, "Force Docker to use specific DNS servers")
	flag.Var(&flDnsSearch, []string{"-dns-search"}, "Force Docker to use specific DNS search domains")
	flag.Var(&flHosts, []string{"H", "-host"}, "The socket(s) to bind to in daemon mode\nspecified using one or more tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.")
	flag.Var(&flGraphOpts, []string{"-storage-opt"}, "Set storage driver options")

=======

	var (
		flHosts     = opts.NewListOpts(api.ValidateHost)
		flVersion   = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
		flDebug     = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
		flTls       = flag.Bool([]string{"-tls"}, false, "Use TLS; implied by tls-verify flags")
		flTlsVerify = flag.Bool([]string{"-tlsverify"}, false, "Use TLS and verify the remote (daemon: verify client, client: verify daemon)")
		flCa        = flag.String([]string{"-tlscacert"}, filepath.Join(client.DockerConfDir, client.DefaultCaFile), "Trust only remotes providing a certificate signed by the CA given here")
		flCert      = flag.String([]string{"-tlscert"}, filepath.Join(client.DockerConfDir, client.DefaultCertFile), "Path to TLS certificate file")
		flKey       = flag.String([]string{"-tlskey"}, filepath.Join(client.DockerConfDir, client.DefaultKeyFile), "Path to TLS key file")
	)
	flag.Var(&flHosts, []string{"H", "-host"}, "The socket(s) to bind to in daemon mode\nspecified using one or more tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.")

>>>>>>> af6ac83... Use docker daemon command instead of -d
	flag.Parse()
	// FIXME: validate daemon flags here

	if *flVersion {
		showVersion()
		return
	}
	if *flDebug {
		os.Setenv("DEBUG", "1")
	}

<<<<<<< HEAD
	if len(flHosts) == 0 {
		defaultHost := os.Getenv("DOCKER_HOST")
		if defaultHost == "" || *flDaemon {
||||||| parent of af6ac83... Use docker daemon command instead of -d
		if defaultHost == "" || *flDaemon {
=======
		if defaultHost == "" {
>>>>>>> af6ac83... Use docker daemon command instead of -d
			// If we do not have a host, default to unix socket
			defaultHost = fmt.Sprintf("unix://%s", api.DEFAULTUNIXSOCKET)
		}
		defaultHost, err := api.ValidateHost(defaultHost)
		if err != nil {
			log.Fatal(err)
		}
		flHosts = append(flHosts, defaultHost)
	}

<<<<<<< HEAD
	if *flDaemon {
		mainDaemon()
		return
	}

	if len(flHosts) > 1 {
		log.Fatal("Please specify only one -H")
	}
	protoAddrParts := strings.SplitN(flHosts[0], "://", 2)

	err := os.MkdirAll(path.Dir(*flTrustKey), 0700)
	if err != nil {
		log.Fatal(err)
||||||| parent of af6ac83... Use docker daemon command instead of -d
	if *bridgeName != "" && *bridgeIp != "" {
		log.Fatal("You specified -b & --bip, mutually exclusive options. Please specify only one.")
	}

	if !*flEnableIptables && !*flInterContainerComm {
		log.Fatal("You specified --iptables=false with --icc=false. ICC uses iptables to function. Please set --icc or --iptables to true.")
	}

	if net.ParseIP(*flDefaultIp) == nil {
		log.Fatalf("Specified --ip=%s is not in correct format \"0.0.0.0\".", *flDefaultIp)
	}

	if *flDebug {
		os.Setenv("DEBUG", "1")
=======
	if *flDebug {
		os.Setenv("DEBUG", "1")
>>>>>>> af6ac83... Use docker daemon command instead of -d
	}
<<<<<<< HEAD
	trustKey, keyErr := libtrust.LoadKeyFile(*flTrustKey)
	if keyErr == libtrust.ErrKeyFileDoesNotExist {
		trustKey, keyErr = libtrust.GenerateECP256PrivateKey()
		if keyErr == nil {
			keyErr = libtrust.SaveKey(*flTrustKey, trustKey)
		}
	}
	if keyErr != nil {
		log.Fatal(keyErr)
	}
	var (
		cli       *client.DockerCli
		tlsConfig tls.Config
	)
	tlsConfig.InsecureSkipVerify = true

	// If we should verify the server, we need to load a trusted ca
	if *flTlsVerify {
		*flTls = true
		certPool := x509.NewCertPool()
		file, err := ioutil.ReadFile(*flCa)
		if err != nil {
			log.Fatalf("Couldn't read ca cert %s: %s", *flCa, err)
		}
		certPool.AppendCertsFromPEM(file)
		tlsConfig.RootCAs = certPool
		tlsConfig.InsecureSkipVerify = false
	}

	// If tls is enabled, try to load and send client certificates
	if *flTls || *flTlsVerify {
		_, errCert := os.Stat(*flCert)
		_, errKey := os.Stat(*flKey)
		if errCert == nil && errKey == nil {
||||||| parent of af6ac83... Use docker daemon command instead of -d

	if *flDaemon {
		if runtime.GOOS != "linux" {
			log.Fatalf("The Docker daemon is only supported on linux")
		}
		if os.Geteuid() != 0 {
			log.Fatalf("The Docker daemon needs to be run as root")
		}

		if flag.NArg() != 0 {
			flag.Usage()
			return
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

		eng := engine.New()
		// Load builtins
		if err := builtins.Register(eng); err != nil {
			log.Fatal(err)
		}

		// handle the pidfile early. https://github.com/dotcloud/docker/issues/6973
		if len(*pidfile) > 0 {
			job := eng.Job("initserverpidfile", *pidfile)
			if err := job.Run(); err != nil {
				log.Fatal(err)
			}
		}

		// load the daemon in the background so we can immediately start
		// the http api so that connections don't fail while the daemon
		// is booting
		go func() {
			// Load plugin: httpapi
			job := eng.Job("initserver")
			// include the variable here too, for the server config
			job.Setenv("Pidfile", *pidfile)
			job.Setenv("Root", realRoot)
			job.SetenvBool("AutoRestart", *flAutoRestart)
			job.SetenvList("Dns", flDns.GetAll())
			job.SetenvList("DnsSearch", flDnsSearch.GetAll())
			job.SetenvBool("EnableIptables", *flEnableIptables)
			job.SetenvBool("EnableIpForward", *flEnableIpForward)
			job.Setenv("BridgeIface", *bridgeName)
			job.Setenv("BridgeIP", *bridgeIp)
			job.Setenv("DefaultIp", *flDefaultIp)
			job.SetenvBool("InterContainerCommunication", *flInterContainerComm)
			job.Setenv("GraphDriver", *flGraphDriver)
			job.SetenvList("GraphOptions", flGraphOpts.GetAll())
			job.Setenv("ExecDriver", *flExecDriver)
			job.SetenvInt("Mtu", *flMtu)
			job.SetenvBool("EnableSelinuxSupport", *flSelinuxEnabled)
			job.SetenvList("Sockets", flHosts.GetAll())
			if err := job.Run(); err != nil {
				log.Fatal(err)
			}
			// after the daemon is done setting up we can tell the api to start
			// accepting connections
			if err := eng.Job("acceptconnections").Run(); err != nil {
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
		job := eng.Job("serveapi", flHosts.GetAll()...)
		job.SetenvBool("Logging", true)
		job.SetenvBool("EnableCors", *flEnableCors)
		job.Setenv("Version", dockerversion.VERSION)
		job.Setenv("SocketGroup", *flSocketGroup)

		job.SetenvBool("Tls", *flTls)
		job.SetenvBool("TlsVerify", *flTlsVerify)
		job.Setenv("TlsCa", *flCa)
		job.Setenv("TlsCert", *flCert)
		job.Setenv("TlsKey", *flKey)
		job.SetenvBool("BufferRequests", true)
		if err := job.Run(); err != nil {
			log.Fatal(err)
		}
	} else {
		if flHosts.Len() > 1 {
			log.Fatal("Please specify only one -H")
		}
		protoAddrParts := strings.SplitN(flHosts.GetAll()[0], "://", 2)

		var (
			cli       *client.DockerCli
			tlsConfig tls.Config
		)
		tlsConfig.InsecureSkipVerify = true

		// If we should verify the server, we need to load a trusted ca
		if *flTlsVerify {
=======
	os.Setenv("Tls", strconv.FormatBool(*flTls))
	os.Setenv("TlsVerify", strconv.FormatBool(*flTlsVerify))
	os.Setenv("TlsCa", *flCa)
	os.Setenv("TlsCert", *flCert)
	os.Setenv("TlsCert", *flKey)

	os.Setenv("flCA", "1")

	if flHosts.Len() > 1 {
		log.Fatal("Please specify only one -H")
	}
	protoAddrParts := strings.SplitN(flHosts.GetAll()[0], "://", 2)

	var (
		cli       *client.DockerCli
		tlsConfig tls.Config
	)
	tlsConfig.InsecureSkipVerify = true

	// If we should verify the server, we need to load a trusted ca
	if *flTlsVerify {
		*flTls = true
		certPool := x509.NewCertPool()
		file, err := ioutil.ReadFile(*flCa)
		if err != nil {
			log.Fatalf("Couldn't read ca cert %s: %s", *flCa, err)
		}
		certPool.AppendCertsFromPEM(file)
		tlsConfig.RootCAs = certPool
		tlsConfig.InsecureSkipVerify = false
	}

	// If tls is enabled, try to load and send client certificates
	if *flTls || *flTlsVerify {
		_, errCert := os.Stat(*flCert)
		_, errKey := os.Stat(*flKey)
		if errCert == nil && errKey == nil {
>>>>>>> af6ac83... Use docker daemon command instead of -d
			*flTls = true
			cert, err := tls.LoadX509KeyPair(*flCert, *flKey)
			if err != nil {
				log.Fatalf("Couldn't load X509 key pair: %s. Key encrypted?", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

<<<<<<< HEAD
	if *flTls || *flTlsVerify {
		cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, trustKey, protoAddrParts[0], protoAddrParts[1], &tlsConfig)
	} else {
		cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, trustKey, protoAddrParts[0], protoAddrParts[1], nil)
	}
||||||| parent of af6ac83... Use docker daemon command instead of -d
		if *flTls || *flTlsVerify {
			cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, protoAddrParts[0], protoAddrParts[1], &tlsConfig)
		} else {
			cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, protoAddrParts[0], protoAddrParts[1], nil)
		}
=======
	if *flTls || *flTlsVerify {
		cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, protoAddrParts[0], protoAddrParts[1], &tlsConfig)
	} else {
		cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, protoAddrParts[0], protoAddrParts[1], nil)
	}
>>>>>>> af6ac83... Use docker daemon command instead of -d

<<<<<<< HEAD
	if err := cli.Cmd(flag.Args()...); err != nil {
		if sterr, ok := err.(*utils.StatusError); ok {
			if sterr.Status != "" {
				log.Println(sterr.Status)
||||||| parent of af6ac83... Use docker daemon command instead of -d
		if err := cli.ParseCommands(flag.Args()...); err != nil {
			if sterr, ok := err.(*utils.StatusError); ok {
				if sterr.Status != "" {
					log.Println(sterr.Status)
				}
				os.Exit(sterr.StatusCode)
=======
	if err := cli.ParseCommands(flag.Args()...); err != nil {
		if sterr, ok := err.(*utils.StatusError); ok {
			if sterr.Status != "" {
				log.Println(sterr.Status)
>>>>>>> af6ac83... Use docker daemon command instead of -d
			}
			os.Exit(sterr.StatusCode)
		}
		log.Fatal(err)
	}
}

func showVersion() {
	fmt.Printf("Docker version %s, build %s\n", dockerversion.VERSION, dockerversion.GITCOMMIT)
}
