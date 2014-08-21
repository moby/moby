package docker

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/client"
)

const (
	errBadCertificate = "remote error: bad certificate"
	errCaUnknown      = "x509: certificate signed by unknown authority"
)

func getTlsConfig(certFile, keyFile string, t *testing.T) *tls.Config {
	certPool := x509.NewCertPool()
	file, err := ioutil.ReadFile("fixtures/https/ca.pem")
	if err != nil {
		t.Fatal(err)
	}
	certPool.AppendCertsFromPEM(file)

	cert, err := tls.LoadX509KeyPair("fixtures/https/"+certFile, "fixtures/https/"+keyFile)
	if err != nil {
		t.Fatalf("Couldn't load X509 key pair: %s", err)
	}
	tlsConfig := &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{cert},
	}
	return tlsConfig
}

// TestHttpsInfo connects via two-way authenticated HTTPS to the info endpoint
func TestHttpsInfo(t *testing.T) {
	cli := client.NewDockerCli(nil, ioutil.Discard, ioutil.Discard, testDaemonProto,
		testDaemonHttpsAddr, getTlsConfig("client-cert.pem", "client-key.pem", t))

	setTimeout(t, "Reading command output time out", 10*time.Second, func() {
		if err := cli.CmdInfo(); err != nil {
			t.Fatal(err)
		}
	})
}

// TestHttpsInfoRogueCert connects via two-way authenticated HTTPS to the info endpoint
// by using a rogue client certificate and checks that it fails with the expected error.
func TestHttpsInfoRogueCert(t *testing.T) {
	cli := client.NewDockerCli(nil, ioutil.Discard, ioutil.Discard, testDaemonProto,
		testDaemonHttpsAddr, getTlsConfig("client-rogue-cert.pem", "client-rogue-key.pem", t))

	setTimeout(t, "Reading command output time out", 10*time.Second, func() {
		err := cli.CmdInfo()
		if err == nil {
			t.Fatal("Expected error but got nil")
		}
		if !strings.Contains(err.Error(), errBadCertificate) {
			t.Fatalf("Expected error: %s, got instead: %s", errBadCertificate, err)
		}
	})
}

// TestHttpsInfoRogueServerCert connects via two-way authenticated HTTPS to the info endpoint
// which provides a rogue server certificate and checks that it fails with the expected error
func TestHttpsInfoRogueServerCert(t *testing.T) {
	cli := client.NewDockerCli(nil, ioutil.Discard, ioutil.Discard, testDaemonProto,
		testDaemonRogueHttpsAddr, getTlsConfig("client-cert.pem", "client-key.pem", t))

	setTimeout(t, "Reading command output time out", 10*time.Second, func() {
		err := cli.CmdInfo()
		if err == nil {
			t.Fatal("Expected error but got nil")
		}

		if !strings.Contains(err.Error(), errCaUnknown) {
			t.Fatalf("Expected error: %s, got instead: %s", errBadCertificate, err)
		}

	})
}
