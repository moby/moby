package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/api/client"
	"github.com/dotcloud/docker/builtins"
	"github.com/dotcloud/docker/dockerversion"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/opts"
	flag "github.com/dotcloud/docker/pkg/mflag"
	"github.com/dotcloud/docker/sysinit"
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

func main() {
	if selfPath := utils.SelfPath(); strings.Contains(selfPath, ".dockerinit") {
		// Running in init mode
		sysinit.SysInit()
		return
	}

	var (
		flVersion   = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
		flDaemon    = flag.Bool([]string{"d", "-daemon"}, false, "Enable daemon mode")
		flDebug     = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
		flDns       = opts.NewListOpts(opts.ValidateIp4Address)
		flDnsSearch = opts.NewListOpts(opts.ValidateDomain)
		flHosts     = opts.NewListOpts(api.ValidateHost)
		flTls       = flag.Bool([]string{"-tls"}, false, "Use TLS; implied by tls-verify flags")
		flTlsVerify = flag.Bool([]string{"-tlsverify"}, false, "Use TLS and verify the remote (daemon: verify client, client: verify daemon)")
		flCa        = flag.String([]string{"-tlscacert"}, dockerConfDir+defaultCaFile, "Trust only remotes providing a certificate signed by the CA given here")
		flCert      = flag.String([]string{"-tlscert"}, dockerConfDir+defaultCertFile, "Path to TLS certificate file")
		flKey       = flag.String([]string{"-tlskey"}, dockerConfDir+defaultKeyFile, "Path to TLS key file")
	)

	// These are still here for the time being to encourage complete compatibility with
	// previous expectations set by the docker CLI. They can be refactored and
	// removed once the client has be transitioned into an engine.Job
	flag.Bool([]string{"r", "-restart"}, true, "Restart previously running containers")
	flag.String([]string{"b", "-bridge"}, "", "Attach containers to a pre-existing network bridge\nuse 'none' to disable container networking")
	flag.String([]string{"#bip", "-bip"}, "", "Use this CIDR notation address for the network bridge's IP, not compatible with -b")
	flag.String([]string{"p", "-pidfile"}, "/var/run/docker.pid", "Path to use for daemon PID file")
	flag.String([]string{"g", "-graph"}, "/var/lib/docker", "Path to use as the root of the docker runtime")
	flag.String([]string{"G", "-group"}, "docker", "Group to assign the unix socket specified by -H when running in daemon mode\nuse '' (the empty string) to disable setting of a group")
	flag.Bool([]string{"#api-enable-cors", "-api-enable-cors"}, false, "Enable CORS headers in the remote API")
	flag.Bool([]string{"#iptables", "-iptables"}, true, "Enable Docker's addition of iptables rules")
	flag.Bool([]string{"#ip-forward", "-ip-forward"}, true, "Enable net.ipv4.ip_forward")
	flag.String([]string{"#ip", "-ip"}, "0.0.0.0", "Default IP address to use when binding container ports")
	flag.Bool([]string{"#icc", "-icc"}, true, "Enable inter-container communication")
	flag.String([]string{"s", "-storage-driver"}, "", "Force the docker runtime to use a specific storage driver")
	flag.String([]string{"e", "-exec-driver"}, "native", "Force the docker runtime to use a specific exec driver")
	flag.Int([]string{"#mtu", "-mtu"}, 0, "Set the containers network MTU\nif no value is provided: default to the default route MTU or 1500 if no default route is available")
	flag.Bool([]string{"-selinux-enabled"}, false, "Enable selinux support")

	flag.Var(&flDns, []string{"#dns", "-dns"}, "Force docker to use specific DNS servers")
	flag.Var(&flDnsSearch, []string{"-dns-search"}, "Force Docker to use specific DNS search domains")
	flag.Var(&flHosts, []string{"H", "-host"}, "The socket(s) to bind to in daemon mode\nspecified using one or more tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.")

	flag.Parse()

	if *flVersion {
		showVersion()
		return
	}

	if *flDebug {
		os.Setenv("DEBUG", "1")
	}

	// Setup an engine
	eng := engine.New()
	// Load builtins
	if err := builtins.Register(eng); err != nil {
		log.Fatal(err)
	}

	if *flDaemon {
		// Serve api

		job := eng.Job("daemon", os.Args[1:]...)
		if err := job.Run(); err != nil {
			log.Fatal(err)
		}
	} else {
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
		} else if flHosts.Len() > 1 {
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
				*flTls = true
				cert, err := tls.LoadX509KeyPair(*flCert, *flKey)
				if err != nil {
					log.Fatalf("Couldn't load X509 key pair: %s. Key encrypted?", err)
				}
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}

		if *flTls || *flTlsVerify {
			cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, protoAddrParts[0], protoAddrParts[1], &tlsConfig)
		} else {
			cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, protoAddrParts[0], protoAddrParts[1], nil)
		}

		if err := cli.ParseCommands(flag.Args()...); err != nil {
			if sterr, ok := err.(*utils.StatusError); ok {
				if sterr.Status != "" {
					log.Println(sterr.Status)
				}
				os.Exit(sterr.StatusCode)
			}
			log.Fatal(err)
		}
	}
}

func showVersion() {
	fmt.Printf("Docker version %s, build %s\n", dockerversion.VERSION, dockerversion.GITCOMMIT)
}
