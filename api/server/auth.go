package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"

	"github.com/docker/libtrust"
)

// NewIdentityAuthTLSConfig creates a tls.Config for the server to use for
// libtrust identity authentication
func NewIdentityAuthTLSConfig(trustKey libtrust.PrivateKey, trustClientsPath, addr string) (*tls.Config, error) {
	tlsConfig := createTLSConfig()

	// Load authorized keys file
	clients, err := libtrust.LoadKeySetFile(trustClientsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to load authorized keys: %s", err)
	}

	// Create a CA pool from authorized keys
	certPool, err := libtrust.GenerateCACertPool(trustKey, clients)
	if err != nil {
		return nil, fmt.Errorf("CA pool generation error: %s", err)
	}
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.ClientCAs = certPool

	// Generate cert
	ips, domains, err := parseAddr(addr)
	if err != nil {
		return nil, err
	}
	// add default docker domain for docker clients to look for
	domains = append(domains, "docker")
	x509Cert, err := libtrust.GenerateSelfSignedServerCert(trustKey, domains, ips)
	if err != nil {
		return nil, fmt.Errorf("certificate generation error: %s", err)
	}
	tlsConfig.Certificates = []tls.Certificate{{
		Certificate: [][]byte{x509Cert.Raw},
		PrivateKey:  trustKey.CryptoPrivateKey(),
		Leaf:        x509Cert,
	}}

	return tlsConfig, nil
}

// NewCertAuthTLSConfig creates a tls.Config for the server to use for
// certificate authentication
func NewCertAuthTLSConfig(caPath, certPath, keyPath string) (*tls.Config, error) {
	tlsConfig := createTLSConfig()

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("Couldn't load X509 key pair (%s, %s): %s. Key encrypted?", certPath, keyPath, err)
	}
	tlsConfig.Certificates = []tls.Certificate{cert}

	// Verify client certificates against a CA?
	if caPath != "" {
		certPool := x509.NewCertPool()
		file, err := ioutil.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("Couldn't read CA certificate: %s", err)
		}
		certPool.AppendCertsFromPEM(file)

		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConfig.ClientCAs = certPool
	}

	return tlsConfig, nil
}

func createTLSConfig() *tls.Config {
	return &tls.Config{
		NextProtos: []string{"http/1.1"},
		// Avoid fallback on insecure SSL protocols
		MinVersion: tls.VersionTLS10,
	}
}

// parseAddr parses an address into an array of IPs and domains
func parseAddr(addr string) ([]net.IP, []string, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, nil, err
	}
	var domains []string
	var ips []net.IP
	ip := net.ParseIP(host)
	if ip != nil {
		ips = []net.IP{ip}
	} else {
		domains = []string{host}
	}
	return ips, domains, nil
}
