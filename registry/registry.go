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
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/timeoutconn"
	"github.com/docker/docker/pkg/transport"
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

// dockerUserAgent is the User-Agent the Docker client uses to identify itself.
// It is populated on init(), comprising version information of different components.
var dockerUserAgent string

func init() {
	httpVersion := make([]useragent.VersionInfo, 0, 6)
	httpVersion = append(httpVersion, useragent.VersionInfo{"docker", dockerversion.VERSION})
	httpVersion = append(httpVersion, useragent.VersionInfo{"go", runtime.Version()})
	httpVersion = append(httpVersion, useragent.VersionInfo{"git-commit", dockerversion.GITCOMMIT})
	if kernelVersion, err := kernel.GetKernelVersion(); err == nil {
		httpVersion = append(httpVersion, useragent.VersionInfo{"kernel", kernelVersion.String()})
	}
	httpVersion = append(httpVersion, useragent.VersionInfo{"os", runtime.GOOS})
	httpVersion = append(httpVersion, useragent.VersionInfo{"arch", runtime.GOARCH})

	dockerUserAgent = useragent.AppendVersions("", httpVersion...)
}

type httpsRequestModifier struct {
	mu        sync.Mutex
	tlsConfig *tls.Config
}

// DRAGONS(tiborvass): If someone wonders why do we set tlsconfig in a roundtrip,
// it's because it's so as to match the current behavior in master: we generate the
// certpool on every-goddam-request. It's not great, but it allows people to just put
// the certs in /etc/docker/certs.d/.../ and let docker "pick it up" immediately. Would
// prefer an fsnotify implementation, but that was out of scope of my refactoring.
func (m *httpsRequestModifier) ModifyRequest(req *http.Request) error {
	var (
		roots   *x509.CertPool
		certs   []tls.Certificate
		hostDir string
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

		if runtime.GOOS == "windows" {
			hostDir = path.Join(os.TempDir(), "/docker/certs.d", req.URL.Host)
		} else {
			hostDir = path.Join("/etc/docker/certs.d", req.URL.Host)
		}
		logrus.Debugf("hostDir: %s", hostDir)
		fs, err := ioutil.ReadDir(hostDir)
		if err != nil && !os.IsNotExist(err) {
			return nil
		}

		for _, f := range fs {
			if strings.HasSuffix(f.Name(), ".crt") {
				if roots == nil {
					roots = x509.NewCertPool()
				}
				logrus.Debugf("crt: %s", hostDir+"/"+f.Name())
				data, err := ioutil.ReadFile(filepath.Join(hostDir, f.Name()))
				if err != nil {
					return err
				}
				roots.AppendCertsFromPEM(data)
			}
			if strings.HasSuffix(f.Name(), ".cert") {
				certName := f.Name()
				keyName := certName[:len(certName)-5] + ".key"
				logrus.Debugf("cert: %s", hostDir+"/"+f.Name())
				if !hasFile(fs, keyName) {
					return fmt.Errorf("Missing key %s for certificate %s", keyName, certName)
				}
				cert, err := tls.LoadX509KeyPair(filepath.Join(hostDir, certName), path.Join(hostDir, keyName))
				if err != nil {
					return err
				}
				certs = append(certs, cert)
			}
			if strings.HasSuffix(f.Name(), ".key") {
				keyName := f.Name()
				certName := keyName[:len(keyName)-4] + ".cert"
				logrus.Debugf("key: %s", hostDir+"/"+f.Name())
				if !hasFile(fs, certName) {
					return fmt.Errorf("Missing certificate %s for key %s", certName, keyName)
				}
			}
		}
		m.mu.Lock()
		m.tlsConfig.RootCAs = roots
		m.tlsConfig.Certificates = certs
		m.mu.Unlock()
	}
	return nil
}

func NewTransport(timeout TimeoutType, secure bool) http.RoundTripper {
	tlsConfig := &tls.Config{
		// Avoid fallback to SSL protocols < TLS1.0
		MinVersion:         tls.VersionTLS10,
		InsecureSkipVerify: !secure,
	}

	tr := &http.Transport{
		DisableKeepAlives: true,
		Proxy:             http.ProxyFromEnvironment,
		TLSClientConfig:   tlsConfig,
	}

	switch timeout {
	case ConnectTimeout:
		tr.Dial = func(proto string, addr string) (net.Conn, error) {
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
		tr.Dial = func(proto string, addr string) (net.Conn, error) {
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
		return transport.NewTransport(tr, &httpsRequestModifier{tlsConfig: tlsConfig})
	}

	return tr
}

// DockerHeaders returns request modifiers that ensure requests have
// the User-Agent header set to dockerUserAgent and that metaHeaders
// are added.
func DockerHeaders(metaHeaders http.Header) []transport.RequestModifier {
	modifiers := []transport.RequestModifier{
		transport.NewHeaderRequestModifier(http.Header{"User-Agent": []string{dockerUserAgent}}),
	}
	if metaHeaders != nil {
		modifiers = append(modifiers, transport.NewHeaderRequestModifier(metaHeaders))
	}
	return modifiers
}

type debugTransport struct {
	http.RoundTripper
	log func(...interface{})
}

func (tr debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	dump, err := httputil.DumpRequestOut(req, false)
	if err != nil {
		tr.log("could not dump request")
	}
	tr.log(string(dump))
	resp, err := tr.RoundTripper.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	dump, err = httputil.DumpResponse(resp, false)
	if err != nil {
		tr.log("could not dump response")
	}
	tr.log(string(dump))
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
