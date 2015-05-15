package registry

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/timeoutconn"
	"github.com/docker/docker/pkg/useragent"
)

var (
	ErrAlreadyExists = errors.New("Image already exists")
	ErrDoesNotExist  = errors.New("Image does not exist")
	errLoginRequired = errors.New("Authentication is required.")
)

type TimeoutType uint32

const (
	NoTimeout TimeoutType = iota
	ReceiveTimeout
	ConnectTimeout
)

type httpsTransport struct {
	*http.Transport
}

// DRAGONS(tiborvass): If someone wonders why do we set tlsconfig in a roundtrip,
// it's because it's so as to match the current behavior in master: we generate the
// certpool on every-goddam-request. It's not great, but it allows people to just put
// the certs in /etc/docker/certs.d/.../ and let docker "pick it up" immediately. Would
// prefer an fsnotify implementation, but that was out of scope of my refactoring.
// TODO: improve things
func (tr *httpsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		roots *x509.CertPool
		certs []tls.Certificate
	)

	if req.URL.Scheme == "https" {
		hasFile := func(files []os.FileInfo, name string) bool {
			for _, f := range files {
				if f.Name() == name {
					return true
				}
			}
			return false
		}

		hostDir := path.Join("/etc/docker/certs.d", req.URL.Host)
		logrus.Debugf("hostDir: %s", hostDir)
		fs, err := ioutil.ReadDir(hostDir)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		for _, f := range fs {
			if strings.HasSuffix(f.Name(), ".crt") {
				if roots == nil {
					roots = x509.NewCertPool()
				}
				logrus.Debugf("crt: %s", hostDir+"/"+f.Name())
				data, err := ioutil.ReadFile(path.Join(hostDir, f.Name()))
				if err != nil {
					return nil, err
				}
				roots.AppendCertsFromPEM(data)
			}
			if strings.HasSuffix(f.Name(), ".cert") {
				certName := f.Name()
				keyName := certName[:len(certName)-5] + ".key"
				logrus.Debugf("cert: %s", hostDir+"/"+f.Name())
				if !hasFile(fs, keyName) {
					return nil, fmt.Errorf("Missing key %s for certificate %s", keyName, certName)
				}
				cert, err := tls.LoadX509KeyPair(path.Join(hostDir, certName), path.Join(hostDir, keyName))
				if err != nil {
					return nil, err
				}
				certs = append(certs, cert)
			}
			if strings.HasSuffix(f.Name(), ".key") {
				keyName := f.Name()
				certName := keyName[:len(keyName)-4] + ".cert"
				logrus.Debugf("key: %s", hostDir+"/"+f.Name())
				if !hasFile(fs, certName) {
					return nil, fmt.Errorf("Missing certificate %s for key %s", certName, keyName)
				}
			}
		}
		if tr.Transport.TLSClientConfig == nil {
			tr.Transport.TLSClientConfig = &tls.Config{
				// Avoid fallback to SSL protocols < TLS1.0
				MinVersion: tls.VersionTLS10,
			}
		}
		tr.Transport.TLSClientConfig.RootCAs = roots
		tr.Transport.TLSClientConfig.Certificates = certs
	}
	return tr.Transport.RoundTrip(req)
}

func NewTransport(timeout TimeoutType, secure bool) http.RoundTripper {
	tlsConfig := tls.Config{
		// Avoid fallback to SSL protocols < TLS1.0
		MinVersion:         tls.VersionTLS10,
		InsecureSkipVerify: !secure,
	}

	transport := &http.Transport{
		DisableKeepAlives: true,
		Proxy:             http.ProxyFromEnvironment,
		TLSClientConfig:   &tlsConfig,
	}

	switch timeout {
	case ConnectTimeout:
		transport.Dial = func(proto string, addr string) (net.Conn, error) {
			// Set the connect timeout to 30 seconds to allow for slower connection
			// times...
			d := net.Dialer{Timeout: 30 * time.Second, DualStack: true}

			conn, err := d.Dial(proto, addr)
			if err != nil {
				return nil, err
			}
			// Set the recv timeout to 10 seconds
			conn.SetDeadline(time.Now().Add(10 * time.Second))
			return conn, nil
		}
	case ReceiveTimeout:
		transport.Dial = func(proto string, addr string) (net.Conn, error) {
			d := net.Dialer{DualStack: true}

			conn, err := d.Dial(proto, addr)
			if err != nil {
				return nil, err
			}
			conn = timeoutconn.New(conn, 1*time.Minute)
			return conn, nil
		}
	}

	if secure {
		// note: httpsTransport also handles http transport
		// but for HTTPS, it sets up the certs
		return &httpsTransport{transport}
	}

	return transport
}

type DockerHeaders struct {
	http.RoundTripper
	Headers http.Header
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}

func (tr *DockerHeaders) RoundTrip(req *http.Request) (*http.Response, error) {
	req = cloneRequest(req)
	httpVersion := make([]useragent.VersionInfo, 0, 4)
	httpVersion = append(httpVersion, useragent.VersionInfo{"docker", dockerversion.VERSION})
	httpVersion = append(httpVersion, useragent.VersionInfo{"go", runtime.Version()})
	httpVersion = append(httpVersion, useragent.VersionInfo{"git-commit", dockerversion.GITCOMMIT})
	if kernelVersion, err := kernel.GetKernelVersion(); err == nil {
		httpVersion = append(httpVersion, useragent.VersionInfo{"kernel", kernelVersion.String()})
	}
	httpVersion = append(httpVersion, useragent.VersionInfo{"os", runtime.GOOS})
	httpVersion = append(httpVersion, useragent.VersionInfo{"arch", runtime.GOARCH})

	userAgent := useragent.AppendVersions(req.UserAgent(), httpVersion...)

	req.Header.Set("User-Agent", userAgent)

	for k, v := range tr.Headers {
		req.Header[k] = v
	}
	return tr.RoundTripper.RoundTrip(req)
}

type debugTransport struct{ http.RoundTripper }

func (tr debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	dump, err := httputil.DumpRequestOut(req, false)
	if err != nil {
		fmt.Println("could not dump request")
	}
	fmt.Println(string(dump))
	resp, err := tr.RoundTripper.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	dump, err = httputil.DumpResponse(resp, false)
	if err != nil {
		fmt.Println("could not dump response")
	}
	fmt.Println(string(dump))
	return resp, err
}

func HTTPClient(transport http.RoundTripper) *http.Client {
	if transport == nil {
		transport = NewTransport(ConnectTimeout, true)
	}

	return &http.Client{
		Transport:     transport,
		CheckRedirect: AddRequiredHeadersToRedirectedRequests,
	}
}

func trustedLocation(req *http.Request) bool {
	var (
		trusteds = []string{"docker.com", "docker.io"}
		hostname = strings.SplitN(req.Host, ":", 2)[0]
	)
	if req.URL.Scheme != "https" {
		return false
	}

	for _, trusted := range trusteds {
		if hostname == trusted || strings.HasSuffix(hostname, "."+trusted) {
			return true
		}
	}
	return false
}

func AddRequiredHeadersToRedirectedRequests(req *http.Request, via []*http.Request) error {
	if via != nil && via[0] != nil {
		if trustedLocation(req) && trustedLocation(via[0]) {
			req.Header = via[0].Header
			return nil
		}
		for k, v := range via[0].Header {
			if k != "Authorization" {
				for _, vv := range v {
					req.Header.Add(k, vv)
				}
			}
		}
	}
	return nil
}
