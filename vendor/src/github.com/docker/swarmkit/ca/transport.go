package ca

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/credentials"

	"golang.org/x/net/context"
)

var (
	// alpnProtoStr are the specified application level protocols for gRPC.
	alpnProtoStr = []string{"h2"}
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "mutablecredentials: Dial timed out" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// MutableTLSCreds is the credentials required for authenticating a connection using TLS.
type MutableTLSCreds struct {
	// Mutex for the tls config
	sync.Mutex
	// TLS configuration
	config *tls.Config
	// TLS Credentials
	tlsCreds credentials.TransportAuthenticator
	// store the subject for easy access
	subject pkix.Name
}

// Info implements the credentials.TransportAuthenticator interface
func (c *MutableTLSCreds) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "tls",
		SecurityVersion:  "1.2",
	}
}

// GetRequestMetadata implements the credentials.TransportAuthenticator interface
func (c *MutableTLSCreds) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return nil, nil
}

// RequireTransportSecurity implements the credentials.TransportAuthenticator interface
func (c *MutableTLSCreds) RequireTransportSecurity() bool {
	return true
}

// ClientHandshake implements the credentials.TransportAuthenticator interface
func (c *MutableTLSCreds) ClientHandshake(addr string, rawConn net.Conn, timeout time.Duration) (net.Conn, credentials.AuthInfo, error) {
	// borrow all the code from the original TLS credentials
	var errChannel chan error
	if timeout != 0 {
		errChannel = make(chan error, 2)
		time.AfterFunc(timeout, func() {
			errChannel <- timeoutError{}
		})
	}
	c.Lock()
	if c.config.ServerName == "" {
		colonPos := strings.LastIndex(addr, ":")
		if colonPos == -1 {
			colonPos = len(addr)
		}
		c.config.ServerName = addr[:colonPos]
	}

	conn := tls.Client(rawConn, c.config)
	// Need to allow conn.Handshake to have access to config,
	// would create a deadlock otherwise
	c.Unlock()
	var err error
	if timeout == 0 {
		err = conn.Handshake()
	} else {
		go func() {
			errChannel <- conn.Handshake()
		}()
		err = <-errChannel
	}
	if err != nil {
		rawConn.Close()
		return nil, nil, err
	}

	return conn, nil, nil
}

// ServerHandshake implements the credentials.TransportAuthenticator interface
func (c *MutableTLSCreds) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	c.Lock()
	conn := tls.Server(rawConn, c.config)
	c.Unlock()
	if err := conn.Handshake(); err != nil {
		rawConn.Close()
		return nil, nil, err
	}

	return conn, credentials.TLSInfo{State: conn.ConnectionState()}, nil
}

// LoadNewTLSConfig replaces the currently loaded TLS config with a new one
func (c *MutableTLSCreds) LoadNewTLSConfig(newConfig *tls.Config) error {
	newSubject, err := GetAndValidateCertificateSubject(newConfig.Certificates)
	if err != nil {
		return err
	}

	c.Lock()
	defer c.Unlock()
	c.subject = newSubject
	c.config = newConfig

	return nil
}

// Role returns the OU for the certificate encapsulated in this TransportAuthenticator
func (c *MutableTLSCreds) Role() string {
	c.Lock()
	defer c.Unlock()

	return c.subject.OrganizationalUnit[0]
}

// Organization returns the O for the certificate encapsulated in this TransportAuthenticator
func (c *MutableTLSCreds) Organization() string {
	c.Lock()
	defer c.Unlock()

	return c.subject.Organization[0]
}

// NodeID returns the CN for the certificate encapsulated in this TransportAuthenticator
func (c *MutableTLSCreds) NodeID() string {
	c.Lock()
	defer c.Unlock()

	return c.subject.CommonName
}

// NewMutableTLS uses c to construct a mutable TransportAuthenticator based on TLS.
func NewMutableTLS(c *tls.Config) (*MutableTLSCreds, error) {
	originalTC := credentials.NewTLS(c)

	if len(c.Certificates) < 1 {
		return nil, fmt.Errorf("invalid configuration: needs at least one certificate")
	}

	subject, err := GetAndValidateCertificateSubject(c.Certificates)
	if err != nil {
		return nil, err
	}

	tc := &MutableTLSCreds{config: c, tlsCreds: originalTC, subject: subject}
	tc.config.NextProtos = alpnProtoStr

	return tc, nil
}

// GetAndValidateCertificateSubject is a helper method to retrieve and validate the subject
// from the x509 certificate underlying a tls.Certificate
func GetAndValidateCertificateSubject(certs []tls.Certificate) (pkix.Name, error) {
	for i := range certs {
		cert := &certs[i]
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			continue
		}
		if len(x509Cert.Subject.OrganizationalUnit) < 1 {
			return pkix.Name{}, fmt.Errorf("no OU found in certificate subject")
		}

		if len(x509Cert.Subject.Organization) < 1 {
			return pkix.Name{}, fmt.Errorf("no organization found in certificate subject")
		}
		if x509Cert.Subject.CommonName == "" {
			return pkix.Name{}, fmt.Errorf("no valid subject names found for TLS configuration")
		}

		return x509Cert.Subject, nil
	}

	return pkix.Name{}, fmt.Errorf("no valid certificates found for TLS configuration")
}
