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
	"github.com/dotcloud/docker/dockerversion"
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

var (
	flVersion   = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
	flDaemon    = flag.Bool([]string{"d", "-daemon"}, false, "Enable daemon mode")
	flGraphOpts opts.ListOpts
	flDebug     = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
	bridgeName  = flag.String([]string{"b", "-bridge"}, "", "Attach containers to a pre-existing network bridge\nuse 'none' to disable container networking")
	bridgeIp    = flag.String([]string{"#bip", "-bip"}, "", "Use this CIDR notation address for the network bridge's IP, not compatible with -b")
	flDns       = opts.NewListOpts(opts.ValidateIp4Address)
	flDnsSearch = opts.NewListOpts(opts.ValidateDomain)
	flHosts     = opts.NewListOpts(api.ValidateHost)
	flTls       = flag.Bool([]string{"-tls"}, false, "Use TLS; implied by tls-verify flags")
	flTlsVerify = flag.Bool([]string{"-tlsverify"}, false, "Use TLS and verify the remote (daemon: verify client, client: verify daemon)")
	flCa        = flag.String([]string{"-tlscacert"}, dockerConfDir+defaultCaFile, "Trust only remotes providing a certificate signed by the CA given here")
	flCert      = flag.String([]string{"-tlscert"}, dockerConfDir+defaultCertFile, "Path to TLS certificate file")
	flKey       = flag.String([]string{"-tlskey"}, dockerConfDir+defaultKeyFile, "Path to TLS key file")
)

func main() {
	if selfPath := utils.SelfPath(); strings.Contains(selfPath, ".dockerinit") {
		// Running in init mode
		trySysInit()
		return
	}

	flag.Var(&flDns, []string{"#dns", "-dns"}, "Force docker to use specific DNS servers")
	flag.Var(&flDnsSearch, []string{"-dns-search"}, "Force Docker to use specific DNS search domains")
	flag.Var(&flHosts, []string{"H", "-host"}, "The socket(s) to bind to in daemon mode\nspecified using one or more tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.")
	flag.Var(&flGraphOpts, []string{"-storage-opt"}, "Set storage driver options")

	flag.Parse()

	if *flVersion {
		showVersion()
		return
	}
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

	if *flDebug {
		os.Setenv("DEBUG", "1")
	}

	if *flDaemon {
		daemonCommand()
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
