package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/dockerversion"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/utils"
	"github.com/docker/libtrust"
)

const (
	defaultTrustKeyFile   = "key.json"
	defaultHostKeysFile   = "allowed_hosts.json"
	defaultClientKeysFile = "authorized_keys.json"
	defaultCaFile         = "ca.pem"
	defaultKeyFile        = "key.pem"
	defaultCertFile       = "cert.pem"
)

func main() {
	if reexec.Init() {
		return
	}

	flag.Parse()
	// FIXME: validate daemon flags here

	if *flVersion {
		showVersion()
		return
	}

	if *flLogLevel != "" {
		lvl, err := log.ParseLevel(*flLogLevel)
		if err != nil {
			log.Fatalf("Unable to parse logging level: %s", *flLogLevel)
		}
		initLogging(lvl)
	} else {
		initLogging(log.InfoLevel)
	}

	// -D, --debug, -l/--log-level=debug processing
	// When/if -D is removed this block can be deleted
	if *flDebug {
		os.Setenv("DEBUG", "1")
		initLogging(log.DebugLevel)
	}

	if len(flHosts) == 0 {
		defaultHost := os.Getenv("DOCKER_HOST")
		if defaultHost == "" || *flDaemon {
			// If we do not have a host, default to unix socket
			defaultHost = fmt.Sprintf("unix://%s", api.DEFAULTUNIXSOCKET)
		}
		defaultHost, err := api.ValidateHost(defaultHost)
		if err != nil {
			log.Fatal(err)
		}
		flHosts = append(flHosts, defaultHost)
	}
	*flTlsVerify = true

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
	}
	trustKey, err := libtrust.LoadKeyFile(*flTrustKey)
	if err == libtrust.ErrKeyFileDoesNotExist {
		trustKey, err = libtrust.GenerateECP256PrivateKey()
		if err != nil {
			log.Fatalf("Error generating key: %s", err)
		}
		if err := libtrust.SaveKey(*flTrustKey, trustKey); err != nil {
			log.Fatalf("Error saving key file: %s", err)
		}
	} else if err != nil {
		log.Fatalf("Error loading key file: %s", err)
	}

	var (
		cli       *client.DockerCli
		tlsConfig tls.Config
	)

	// Load known hosts
	knownHosts, err := libtrust.LoadKeySetFile(*flTrustHosts)
	if err != nil {
		log.Fatalf("Could not load trusted hosts file: %s", err)
	}

	// Regardless of whether the user sets it to true or false, if they
	// specify --tlsverify at all then we need to turn on tls
	if flag.IsSet("-tlsverify") {
		*flTls = true
	}

	// If we should verify the server, we need to load a trusted ca
	if *flTlsVerify {
		allowedHosts, err := libtrust.FilterByHosts(knownHosts, protoAddrParts[1], false)
		if err != nil {
			log.Fatalf("Error filtering hosts: %s", err)
		}
		certPool, err := libtrust.GenerateCACertPool(trustKey, allowedHosts)
		if err != nil {
			log.Fatalf("Could not create CA pool: %s", err)
		}
		if *flCa != "" {
			file, err := ioutil.ReadFile(*flCa)
			if err != nil {
				if !os.IsNotExist(err) {
					log.Fatalf("Couldn't read ca cert %s: %s", *flCa, err)
				} else {
					tlsConfig.ServerName = "docker"
				}
			} else {
				certPool.AppendCertsFromPEM(file)
			}
		} else {
			tlsConfig.ServerName = "docker"
		}
		tlsConfig.RootCAs = certPool
		tlsConfig.InsecureSkipVerify = false
	}

	// If tls is enabled, try to load and send client certificates
	if *flTls || *flTlsVerify {
		*flTls = true
		cert, err := tls.LoadX509KeyPair(*flCert, *flKey)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Fatalf("Couldn't load X509 key pair: %s. Key encrypted?", err)
			} else {
				x509Cert, certErr := libtrust.GenerateSelfSignedClientCert(trustKey)
				if certErr != nil {
					log.Fatalf("Certificate generation error: %s", certErr)
				}
				cert = tls.Certificate{
					Certificate: [][]byte{x509Cert.Raw},
					PrivateKey:  trustKey.CryptoPrivateKey(),
					Leaf:        x509Cert,
				}
			}
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if protoAddrParts[0] != "unix" && (!*flInsecure) {
		savedInsecure := tlsConfig.InsecureSkipVerify
		tlsConfig.InsecureSkipVerify = true
		testConn, connErr := tls.Dial(protoAddrParts[0], protoAddrParts[1], &tlsConfig)
		if connErr != nil {
			log.Fatalf("TLS Handshake error: %s", connErr)
		}
		opts := x509.VerifyOptions{
			Roots:         tlsConfig.RootCAs,
			CurrentTime:   time.Now(),
			DNSName:       tlsConfig.ServerName,
			Intermediates: x509.NewCertPool(),
		}

		certs := testConn.ConnectionState().PeerCertificates
		for i, cert := range certs {
			if i == 0 {
				continue
			}
			opts.Intermediates.AddCert(cert)
		}
		_, err = certs[0].Verify(opts)
		if err != nil {
			if _, ok := err.(x509.UnknownAuthorityError); ok {
				pubKey, err := libtrust.FromCryptoPublicKey(certs[0].PublicKey)
				if err != nil {
					log.Fatalf("Error extracting public key from certificate: %s", err)
				}

				if promptUnknownKey(pubKey, protoAddrParts[1]) {
					pubKey.AddExtendedField("hosts", []string{protoAddrParts[1]})
					err = libtrust.AddKeySetFile(*flTrustHosts, pubKey)
					if err != nil {
						log.Fatalf("Error saving updated host keys file: %s", err)
					}

					ca, err := libtrust.GenerateCACert(trustKey, pubKey)
					if err != nil {
						log.Fatalf("Error generating CA: %s", err)
					}
					tlsConfig.RootCAs.AddCert(ca)
				} else {
					log.Fatalf("Cancelling request due to invalid certificate")
				}
			} else {
				log.Fatalf("TLS verification error: %s", connErr)
			}
		}

		testConn.Close()
		tlsConfig.InsecureSkipVerify = savedInsecure

		// Avoid fallback to SSL protocols < TLS1.0
		tlsConfig.MinVersion = tls.VersionTLS10
	}

	if protoAddrParts[0] == "unix" || *flInsecure {
		cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, trustKey, protoAddrParts[0], protoAddrParts[1], nil)
	} else {
		cli = client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, trustKey, protoAddrParts[0], protoAddrParts[1], &tlsConfig)
	}

	if err := cli.Cmd(flag.Args()...); err != nil {
		if sterr, ok := err.(*utils.StatusError); ok {
			if sterr.Status != "" {
				log.Println(sterr.Status)
			}
			os.Exit(sterr.StatusCode)
		}
		log.Fatal(err)
	}
}

func promptUnknownKey(key libtrust.PublicKey, host string) bool {
	fmt.Printf("The authenticity of host %q can't be established.\nRemote key ID %s\n", host, key.KeyID())
	fmt.Printf("Are you sure you want to continue connecting (yes/no)? ")
	reader := bufio.NewReader(os.Stdin)
	line, _, err := reader.ReadLine()
	if err != nil {
		log.Fatalf("Error reading input: %s", err)
	}
	input := strings.TrimSpace(strings.ToLower(string(line)))
	return input == "yes" || input == "y"
}

func showVersion() {
	fmt.Printf("Docker version %s, build %s\n", dockerversion.VERSION, dockerversion.GITCOMMIT)
}
