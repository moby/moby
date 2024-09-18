package resolver

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/containerd/remotes/docker"
	"github.com/pkg/errors"

	"github.com/moby/buildkit/util/resolver/config"
	"github.com/moby/buildkit/util/tracing"
)

const (
	defaultPath = "/v2"
)

func fillInsecureOpts(host string, c config.RegistryConfig, h docker.RegistryHost) (*docker.RegistryHost, error) {
	tc, err := loadTLSConfig(c)
	if err != nil {
		return nil, err
	}
	var isHTTP bool

	if c.PlainHTTP != nil && *c.PlainHTTP {
		isHTTP = true
	}
	if c.PlainHTTP == nil {
		if ok, _ := docker.MatchLocalhost(host); ok {
			isHTTP = true
		}
	}

	httpsTransport := newDefaultTransport()
	httpsTransport.TLSClientConfig = tc

	if c.Insecure != nil && *c.Insecure {
		h2 := h

		var transport http.RoundTripper = httpsTransport
		if isHTTP {
			// TODO: Replace this with [docker.NewHTTPFallback] once
			// backported to vendored version of containerd
			transport = &httpFallback{super: transport}
		}
		h2.Client = &http.Client{
			Transport: tracing.NewTransport(transport),
		}
		tc.InsecureSkipVerify = true
		return &h2, nil
	} else if isHTTP {
		h2 := h
		h2.Scheme = "http"
		return &h2, nil
	}

	h.Client = &http.Client{
		Transport: tracing.NewTransport(httpsTransport),
	}
	return &h, nil
}

func loadTLSConfig(c config.RegistryConfig) (*tls.Config, error) {
	for _, d := range c.TLSConfigDir {
		fs, err := os.ReadDir(d)
		if err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, os.ErrPermission) {
			return nil, errors.WithStack(err)
		}
		for _, f := range fs {
			if strings.HasSuffix(f.Name(), ".crt") {
				c.RootCAs = append(c.RootCAs, filepath.Join(d, f.Name()))
			}
			if strings.HasSuffix(f.Name(), ".cert") {
				c.KeyPairs = append(c.KeyPairs, config.TLSKeyPair{
					Certificate: filepath.Join(d, f.Name()),
					Key:         filepath.Join(d, strings.TrimSuffix(f.Name(), ".cert")+".key"),
				})
			}
		}
	}

	tc := &tls.Config{}
	if len(c.RootCAs) > 0 {
		systemPool, err := x509.SystemCertPool()
		if err != nil {
			if runtime.GOOS == "windows" {
				systemPool = x509.NewCertPool()
			} else {
				return nil, errors.Wrapf(err, "unable to get system cert pool")
			}
		}
		tc.RootCAs = systemPool
	}

	for _, p := range c.RootCAs {
		dt, err := os.ReadFile(p)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read %s", p)
		}
		tc.RootCAs.AppendCertsFromPEM(dt)
	}

	for _, kp := range c.KeyPairs {
		cert, err := tls.LoadX509KeyPair(kp.Certificate, kp.Key)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load keypair for %s", kp.Certificate)
		}
		tc.Certificates = append(tc.Certificates, cert)
	}
	return tc, nil
}

// NewRegistryConfig converts registry config to docker.RegistryHosts callback
func NewRegistryConfig(m map[string]config.RegistryConfig) docker.RegistryHosts {
	return docker.Registries(
		func(host string) ([]docker.RegistryHost, error) {
			c, ok := m[host]
			if !ok {
				return nil, nil
			}

			var out []docker.RegistryHost

			for _, rawMirror := range c.Mirrors {
				h := newMirrorRegistryHost(rawMirror)
				mirrorHost := h.Host
				host, err := fillInsecureOpts(mirrorHost, m[mirrorHost], h)
				if err != nil {
					return nil, err
				}

				out = append(out, *host)
			}

			if host == "docker.io" {
				host = "registry-1.docker.io"
			}

			h := docker.RegistryHost{
				Scheme:       "https",
				Client:       newDefaultClient(),
				Host:         host,
				Path:         "/v2",
				Capabilities: docker.HostCapabilityPush | docker.HostCapabilityPull | docker.HostCapabilityResolve,
			}

			hosts, err := fillInsecureOpts(host, c, h)
			if err != nil {
				return nil, err
			}

			out = append(out, *hosts)

			return out, nil
		},
		docker.ConfigureDefaultRegistries(
			docker.WithClient(newDefaultClient()),
			docker.WithPlainHTTP(docker.MatchLocalhost),
		),
	)
}

func newMirrorRegistryHost(mirror string) docker.RegistryHost {
	mirrorHost, mirrorPath := extractMirrorHostAndPath(mirror)
	path := path.Join(defaultPath, mirrorPath)
	h := docker.RegistryHost{
		Scheme:       "https",
		Client:       newDefaultClient(),
		Host:         mirrorHost,
		Path:         path,
		Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve,
	}

	return h
}

func newDefaultClient() *http.Client {
	return &http.Client{
		Transport: tracing.NewTransport(newDefaultTransport()),
	}
}

// newDefaultTransport is for pull or push client
//
// NOTE: For push, there must disable http2 for https because the flow control
// will limit data transfer. The net/http package doesn't provide http2 tunable
// settings which limits push performance.
//
// REF: https://github.com/golang/go/issues/14077
func newDefaultTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		MaxIdleConns:          30,
		IdleConnTimeout:       120 * time.Second,
		MaxIdleConnsPerHost:   4,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
		TLSNextProto:          make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}
}

type httpFallback struct {
	super   http.RoundTripper
	host    string
	hostMut sync.Mutex
}

func (f *httpFallback) RoundTrip(r *http.Request) (*http.Response, error) {
	f.hostMut.Lock()
	// Skip the HTTPS call only if the same host had previously fell back
	tryHTTPSFirst := f.host != r.URL.Host
	f.hostMut.Unlock()

	if tryHTTPSFirst {
		resp, err := f.super.RoundTrip(r)
		if !isTLSError(err) && !isPortError(err, r.URL.Host) {
			return resp, err
		}
	}

	plainHTTPUrl := *r.URL
	plainHTTPUrl.Scheme = "http"

	plainHTTPRequest := *r
	plainHTTPRequest.URL = &plainHTTPUrl

	// We tried HTTPS first but it failed.
	// Mark the host so we don't try HTTPS for this host next time
	// and refresh the request body.
	if tryHTTPSFirst {
		f.hostMut.Lock()
		f.host = r.URL.Host
		f.hostMut.Unlock()

		// update body on the second attempt
		if r.Body != nil && r.GetBody != nil {
			body, err := r.GetBody()
			if err != nil {
				return nil, err
			}
			plainHTTPRequest.Body = body
		}
	}

	return f.super.RoundTrip(&plainHTTPRequest)
}

func isTLSError(err error) bool {
	if err == nil {
		return false
	}
	var tlsErr tls.RecordHeaderError
	if errors.As(err, &tlsErr) && string(tlsErr.RecordHeader[:]) == "HTTP/" {
		return true
	}
	if strings.Contains(err.Error(), "TLS handshake timeout") {
		return true
	}

	return false
}

func isPortError(err error, host string) bool {
	if errors.Is(err, syscall.ECONNREFUSED) || os.IsTimeout(err) {
		if _, port, _ := net.SplitHostPort(host); port != "" {
			// Port is specified, will not retry on different port with scheme change
			return false
		}
		return true
	}

	return false
}
