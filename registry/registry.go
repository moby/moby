// Package registry contains client primitives to interact with a remote Docker registry.
package registry

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/log"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/go-connections/tlsconfig"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// HostCertsDir returns the config directory for a specific host.
//
// Deprecated: this function was only used internally, and will be removed in a future release.
func HostCertsDir(hostname string) string {
	return hostCertsDir(hostname)
}

// hostCertsDir returns the config directory for a specific host.
func hostCertsDir(hostname string) string {
	return filepath.Join(CertsDir(), cleanPath(hostname))
}

// newTLSConfig constructs a client TLS configuration based on server defaults
func newTLSConfig(ctx context.Context, hostname string, isSecure bool) (*tls.Config, error) {
	// PreferredServerCipherSuites should have no effect
	tlsConfig := tlsconfig.ServerDefault()
	tlsConfig.InsecureSkipVerify = !isSecure

	if isSecure {
		hostDir := hostCertsDir(hostname)
		log.G(ctx).Debugf("hostDir: %s", hostDir)
		if err := loadTLSConfig(ctx, hostDir, tlsConfig); err != nil {
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
	return loadTLSConfig(context.TODO(), directory, tlsConfig)
}

// loadTLSConfig reads the directory for TLS certificates including roots and
// certificate pairs, and updates the provided TLS configuration.
func loadTLSConfig(ctx context.Context, directory string, tlsConfig *tls.Config) error {
	fs, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return invalidParam(err)
	}

	for _, f := range fs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		switch filepath.Ext(f.Name()) {
		case ".crt":
			if tlsConfig.RootCAs == nil {
				systemPool, err := tlsconfig.SystemCertPool()
				if err != nil {
					return invalidParamWrapf(err, "unable to get system cert pool")
				}
				tlsConfig.RootCAs = systemPool
			}
			fileName := filepath.Join(directory, f.Name())
			log.G(ctx).Debugf("crt: %s", fileName)
			data, err := os.ReadFile(fileName)
			if err != nil {
				return err
			}
			tlsConfig.RootCAs.AppendCertsFromPEM(data)
		case ".cert":
			certName := f.Name()
			keyName := certName[:len(certName)-5] + ".key"
			log.G(ctx).Debugf("cert: %s", filepath.Join(directory, certName))
			if !hasFile(fs, keyName) {
				return invalidParamf("missing key %s for client certificate %s. CA certificates must use the extension .crt", keyName, certName)
			}
			cert, err := tls.LoadX509KeyPair(filepath.Join(directory, certName), filepath.Join(directory, keyName))
			if err != nil {
				return err
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
		case ".key":
			keyName := f.Name()
			certName := keyName[:len(keyName)-4] + ".cert"
			log.G(ctx).Debugf("key: %s", filepath.Join(directory, keyName))
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
func newTransport(tlsConfig *tls.Config) http.RoundTripper {
	if tlsConfig == nil {
		tlsConfig = tlsconfig.ServerDefault()
	}

	return otelhttp.NewTransport(
		&http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     tlsConfig,
			// TODO(dmcgowan): Call close idle connections when complete and use keep alive
			DisableKeepAlives: true,
		},
	)
}
