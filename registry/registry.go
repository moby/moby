// Package registry contains client primitives to interact with a remote Docker registry.
package registry // import "github.com/docker/docker/registry"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ThalesIgnite/crypto11"
	"github.com/containerd/log"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/pkg/errors"
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

	// PKCS11 support: You MUST compile with `make dynbinary`
	// for this to work. You can then put your locally built
	// binary into service with:
	//   sudo systemctl stop docker.service && sudo cp bundles/dynbinary-daemon/dockerd /usr/sbin/dockerd 
	//
	// We put the configuration into environment variables as a quick
	// and dirty way to demonstrate that this works.
	//
	// The docker.service file on Debian installs includes:
	//   EnvironmentFile=-/etc/default/docker
	//
	// So you can put something like the following into /etc/default/docker:
	//
	//   PKCS11_LABEL=my.cert.name
	//   PKCS11_PIN=112233
	//   PKCS11_MODULE=/usr/lib/x86_64-linux-gnu/opensc-pkcs11.so
	//
	// You can use only one of PKCS11_SERIAL, PKCS11_SLOT or
	// PKCS11_LABEL.
	//
	// Then `systemctl restart docker.service` to make it take effect.	
	pkcs11_module := os.Getenv("PKCS11_MODULE")
	if pkcs11_module != "" {
		cfg := &crypto11.Config{
			Path:       pkcs11_module,
			TokenSerial: os.Getenv("PKCS11_SERIAL"),
			TokenLabel: os.Getenv("PKCS11_LABEL"),
			Pin:        os.Getenv("PKCS11_PIN"),
		}
		if os.Getenv("PKCS11_SLOT") != "" {
			slot, err := strconv.Atoi(os.Getenv("PKCS11_SLOT"))
			if err != nil {
				return nil, err
			}
			cfg.SlotNumber = &slot
		}

		ctx11, err := crypto11.Configure(cfg)
		if err != nil {
			log.G(context.TODO()).WithError(err).Warn("Could not open PKCS11 context.")
			return nil, err
		}

		certificates, err := ctx11.FindAllPairedCertificates()
		if err != nil {
			return nil, err
		}

		if len(certificates) == 0 {
			return nil, errors.New("no certificate found in your pkcs11 device")
		}

		if len(certificates) > 1 {
			return nil, errors.New("got more than one certificate")
		}

		theCert, err := x509.ParseCertificate(certificates[0].Certificate[0])
		if err != nil {
			return nil, err
		}
		log.G(context.TODO()).Debugf("Certificate: %v", theCert)

		tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return &certificates[0], nil }
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
