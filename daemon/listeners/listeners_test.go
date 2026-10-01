package listeners

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http/httptest"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestTCPListenerALPN(t *testing.T) {
	certServer := httptest.NewTLSServer(nil)
	defer certServer.Close()
	roots := x509.NewCertPool()
	roots.AddCert(certServer.Certificate())

	for _, proto := range []string{"h2", "http/1.1", ""} {
		name := proto
		if name == "" {
			name = "no_alpn"
		}
		t.Run(name, func(t *testing.T) {
			config := &tls.Config{
				Certificates: certServer.TLS.Certificates,
				NextProtos:   []string{"h2", "http/1.1"},
				MinVersion:   tls.VersionTLS12,
			}
			listeners, err := Init("tcp", "127.0.0.1:0", "", config)
			assert.NilError(t, err)
			listener := listeners[0]
			defer listener.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			handshake := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					handshake <- err
					return
				}
				defer conn.Close()
				handshake <- conn.(*tls.Conn).HandshakeContext(ctx)
			}()

			clientConfig := &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
			if proto != "" {
				clientConfig.NextProtos = []string{proto}
			}
			dialer := tls.Dialer{Config: clientConfig}
			conn, err := dialer.DialContext(ctx, "tcp", listener.Addr().String())
			assert.NilError(t, err)
			defer conn.Close()
			assert.NilError(t, <-handshake)
			assert.Equal(t, conn.(*tls.Conn).ConnectionState().NegotiatedProtocol, proto)
			assert.DeepEqual(t, config.NextProtos, []string{"h2", "http/1.1"})
		})
	}
}
