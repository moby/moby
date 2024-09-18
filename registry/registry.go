// Package registry contains client primitives to interact with a remote Docker registry.
package registry // import "github.com/docker/docker/registry"

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/log"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/go-connections/tlsconfig"
)

// HostCertsDir returns the config directory for a specific host.
func HostCertsDir(hostname string) string {
	return filepath.Join(CertsDir(), cleanPath(hostname))
}

// newTLSConfig constructs a client TLS configuration based on server defaults
func newTLSConfig(hostname string, isSecure bool) (*tls.Config, error) {
	// PreferredServerCipherSuites should have no effect
	tlsConfig := tlsconfig.ServerDefault()

	tlsConfig.InsecureSkipVerify = !isSecure

	if isSecure && CertsDir() != "" {
		hostDir := HostCertsDir(hostname)
		log.G(context.TODO()).Debugf("hostDir: %s", hostDir)
		if err := ReadCertsDirectory(tlsConfig, hostDir); err != nil {
			return nil, err
		}
	}

	return tlsConfig, nil
}

func hasFile(files []os.DirEntry, name string) bool {
	for _, f := range files {
		if f.Name() == name {
			return true
		}
	}
	return false
}

// ReadCertsDirectory reads the directory for TLS certificates
// including roots and certificate pairs and updates the
// provided TLS configuration.
func ReadCertsDirectory(tlsConfig *tls.Config, directory string) error {
	fs, err := os.ReadDir(directory)
	if err != nil && !os.IsNotExist(err) {
		return invalidParam(err)
	}

	for _, f := range fs {
		if strings.HasSuffix(f.Name(), ".crt") {
			if tlsConfig.RootCAs == nil {
				systemPool, err := tlsconfig.SystemCertPool()
				if err != nil {
					return invalidParamWrapf(err, "unable to get system cert pool")
				}
				tlsConfig.RootCAs = systemPool
			}
			log.G(context.TODO()).Debugf("crt: %s", filepath.Join(directory, f.Name()))
			data, err := os.ReadFile(filepath.Join(directory, f.Name()))
			if err != nil {
				return err
			}
			tlsConfig.RootCAs.AppendCertsFromPEM(data)
		}
		if strings.HasSuffix(f.Name(), ".cert") {
			certName := f.Name()
			keyName := certName[:len(certName)-5] + ".key"
			log.G(context.TODO()).Debugf("cert: %s", filepath.Join(directory, f.Name()))
			if !hasFile(fs, keyName) {
				return invalidParamf("missing key %s for client certificate %s. CA certificates must use the extension .crt", keyName, certName)
			}
			cert, err := tls.LoadX509KeyPair(filepath.Join(directory, certName), filepath.Join(directory, keyName))
			if err != nil {
				return err
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
		}
		if strings.HasSuffix(f.Name(), ".key") {
			keyName := f.Name()
			certName := keyName[:len(keyName)-4] + ".cert"
			log.G(context.TODO()).Debugf("key: %s", filepath.Join(directory, f.Name()))
			if !hasFile(fs, certName) {
				return invalidParamf("missing client certificate %s for key %s", certName, keyName)
			}
		}
	}

	return nil
}

// Headers returns request modifiers with a User-Agent and metaHeaders
func Headers(userAgent string, metaHeaders http.Header) []transport.RequestModifier {
	modifiers := []transport.RequestModifier{}
	if userAgent != "" {
		modifiers = append(modifiers, transport.NewHeaderRequestModifier(http.Header{
			"User-Agent": []string{userAgent},
		}))
	}
	if metaHeaders != nil {
		modifiers = append(modifiers, transport.NewHeaderRequestModifier(metaHeaders))
	}
	return modifiers
}

// newTransport returns a new HTTP transport. If tlsConfig is nil, it uses the
// default TLS configuration.
func newTransport(tlsConfig *tls.Config) *http.Transport {
	if tlsConfig == nil {
		tlsConfig = tlsconfig.ServerDefault()
	}

	direct := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	return &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         direct.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		// TODO(dmcgowan): Call close idle connections when complete and use keep alive
		DisableKeepAlives: true,
	}
}
