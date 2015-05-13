package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/term"
)

const (
	defaultTrustKeyFile = "key.json"
	defaultCaFile       = "ca.pem"
	defaultKeyFile      = "key.pem"
	defaultCertFile     = "cert.pem"
)

func main() {
	if reexec.Init() {
		return
	}

	// Set terminal emulation based on platform as required.
	stdin, stdout, stderr := term.StdStreams()

	initLogging(stderr)

	flag.Parse()
	// FIXME: validate daemon flags here

	if *flVersion {
		showVersion()
		return
	}

	if *flLogLevel != "" {
		lvl, err := logrus.ParseLevel(*flLogLevel)
		if err != nil {
			logrus.Fatalf("Unable to parse logging level: %s", *flLogLevel)
		}
		setLogLevel(lvl)
	} else {
		setLogLevel(logrus.InfoLevel)
	}

	if *flDebug {
		os.Setenv("DEBUG", "1")
	}

	if len(flHosts) == 0 {
		defaultHost := os.Getenv("DOCKER_HOST")
		if defaultHost == "" || *flDaemon {
			if runtime.GOOS != "windows" {
				// If we do not have a host, default to unix socket
				defaultHost = fmt.Sprintf("unix://%s", opts.DefaultUnixSocket)
			} else {
				// If we do not have a host, default to TCP socket on Windows
				defaultHost = fmt.Sprintf("tcp://%s:%d", opts.DefaultHTTPHost, opts.DefaultHTTPPort)
			}
		}
		defaultHost, err := opts.ValidateHost(defaultHost)
		if err != nil {
			logrus.Fatal(err)
		}
		flHosts = append(flHosts, defaultHost)
	}

	setDefaultConfFlag(flTrustKey, defaultTrustKeyFile)

	if *flDaemon {
		if *flHelp {
			flag.Usage()
			return
		}
		mainDaemon()
		return
	}

	if len(flHosts) > 1 {
		logrus.Fatal("Please specify only one -H")
	}
	protoAddrParts := strings.SplitN(flHosts[0], "://", 2)

	var (
		cli       *client.DockerCli
		tlsConfig tls.Config
	)
	tlsConfig.InsecureSkipVerify = true

	// Regardless of whether the user sets it to true or false, if they
	// specify --tlsverify at all then we need to turn on tls
	if flag.IsSet("-tlsverify") {
		*flTls = true
	}

	// If we should verify the server, we need to load a trusted ca
	if *flTlsVerify {
		certPool := x509.NewCertPool()
		file, err := ioutil.ReadFile(*flCa)
		if err != nil {
			logrus.Fatalf("Couldn't read ca cert %s: %s", *flCa, err)
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
				logrus.Fatalf("Couldn't load X509 key pair: %q. Make sure the key is encrypted", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
		// Avoid fallback to SSL protocols < TLS1.0
		tlsConfig.MinVersion = tls.VersionTLS10
	}

	if *flTls || *flTlsVerify {
		cli = client.NewDockerCli(stdin, stdout, stderr, *flTrustKey, protoAddrParts[0], protoAddrParts[1], &tlsConfig)
	} else {
		cli = client.NewDockerCli(stdin, stdout, stderr, *flTrustKey, protoAddrParts[0], protoAddrParts[1], nil)
	}

	if err := cli.Cmd(flag.Args()...); err != nil {
		if sterr, ok := err.(client.StatusError); ok {
			if sterr.Status != "" {
				logrus.Println(sterr.Status)
			}
			os.Exit(sterr.StatusCode)
		}
		logrus.Fatal(err)
	}
}

func showVersion() {
	fmt.Printf("Docker version %s, build %s\n", dockerversion.VERSION, dockerversion.GITCOMMIT)
}
