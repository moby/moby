package registry

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/utils"
	"github.com/go-fsnotify/fsnotify"
)

var (
	caPool       *x509.CertPool
	clientCerts  []tls.Certificate
	certLock     sync.RWMutex
	certWatcher  *fsnotify.Watcher
	certsDirname = "/etc/docker/certs.d"
)

func init() {
	// Setup initial certificate pool and client certificates.
	loadCerts(watchCertDirs())
}

type TimeoutType uint32

const (
	NoTimeout TimeoutType = iota
	ReceiveTimeout
	ConnectTimeout
)

// Client should handle http requests for all registry sessions.
type Client struct {
	httpClient *http.Client
	isSecure   bool
}

// Do handles making an http request and ensures that
// Auth headers are never sent over an insecure channel.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	authHeader := req.Header.Get("Authorization")
	if authHeader != "" && !(c.isSecure && req.URL.Scheme == "https") {
		return nil, fmt.Errorf("cannot send auth credentials over insecure channel %q: %s", authHeader, req.URL)
	}

	return c.httpClient.Do(req)
}

// NewClient prepares and returns an HTTP Client to use with this endpoint.
func (e *Endpoint) NewClient(jar http.CookieJar, timeout TimeoutType) *Client {
	return newClient(jar, timeout, e.IsSecure)
}

func newClient(jar http.CookieJar, timeout TimeoutType, secure bool) *Client {
	certLock.RLock()
	tlsConfig := tls.Config{
		RootCAs: caPool,
		// Avoid fallback to SSL protocols < TLS1.0
		MinVersion:   tls.VersionTLS10,
		Certificates: clientCerts,
	}
	certLock.RUnlock()

	if !secure {
		tlsConfig.InsecureSkipVerify = true
	}

	httpTransport := &http.Transport{
		DisableKeepAlives: true,
		Proxy:             http.ProxyFromEnvironment,
		TLSClientConfig:   &tlsConfig,
	}

	switch timeout {
	case ConnectTimeout:
		httpTransport.Dial = func(proto string, addr string) (net.Conn, error) {
			// Set the connect timeout to 5 seconds
			d := net.Dialer{Timeout: 5 * time.Second, DualStack: true}

			conn, err := d.Dial(proto, addr)
			if err != nil {
				return nil, err
			}
			// Set the recv timeout to 10 seconds
			conn.SetDeadline(time.Now().Add(10 * time.Second))
			return conn, nil
		}
	case ReceiveTimeout:
		httpTransport.Dial = func(proto string, addr string) (net.Conn, error) {
			d := net.Dialer{DualStack: true}

			conn, err := d.Dial(proto, addr)
			if err != nil {
				return nil, err
			}
			conn = utils.NewTimeoutConn(conn, 1*time.Minute)
			return conn, nil
		}
	}

	return &Client{
		httpClient: &http.Client{
			Transport:     httpTransport,
			CheckRedirect: AddRequiredHeadersToRedirectedRequests,
			Jar:           jar,
		},
		isSecure: secure,
	}
}

func getCertDirs() (certDirs []string) {
	if err := os.MkdirAll(certsDirname, os.FileMode(0755)); err != nil {
		log.Fatalf("unable to make docker registry certs directory %s: %s", certsDirname, err)
	}

	certDirs = []string{certsDirname}

	// List directory entries to find all immediate subdirectories.
	dirEntries, err := ioutil.ReadDir(certsDirname)
	if err != nil {
		log.Fatalf("unable to read certificate directory %s: %s", certsDirname, err)
	}

	for _, entry := range dirEntries {
		if entry.IsDir() {
			certDirs = append(certDirs, path.Join(certsDirname, entry.Name()))
		}
	}

	return
}

func watchCertDirs() (certDirs []string) {
	if certWatcher != nil {
		certWatcher.Close()
	}

	certDirs = getCertDirs()

	certWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("unable to create certificate directory notifier: %s", err)
	}

	for _, certDir := range certDirs {
		if err = certWatcher.Add(certDir); err != nil {
			log.Errorf("unable to watch cert directory %s: %s", certDir, err)
		}
	}

	go handleCertDirEvent(certWatcher)

	return
}

func handleCertDirEvent(watcher *fsnotify.Watcher) {
	var err error

	select {
	case _ = <-watcher.Events:
	case err = <-watcher.Errors:
	}

	if err != nil {
		log.Errorf("error watching for certificate dir events: %s", err)
	}

	// There was some filesystem event in one of the watched
	// certificate directories. We should reload all certificates
	// and reset the watcher.
	loadCerts(watchCertDirs())
}

func loadCerts(certDirs []string) {
	newCAPool, newClientCerts := prepareCerts(certDirs)

	certLock.Lock()
	caPool, clientCerts = newCAPool, newClientCerts
	certLock.Unlock()
}

func addCACertToPool(pemFilename string, pool *x509.CertPool) (err error) {
	log.Debugf("loading CA certificate file: %s", pemFilename)

	var certPEM []byte
	if certPEM, err = ioutil.ReadFile(pemFilename); err != nil {
		return err
	}

	pool.AppendCertsFromPEM(certPEM)

	return nil
}

func addClientCert(certFilename string, dirFileSet map[string]struct{}, certs *[]tls.Certificate) (err error) {
	log.Debugf("loading client certificate file: %s", certFilename)

	certName := path.Base(certFilename)
	keyName := strings.TrimSuffix(certName, ".cert") + ".key"
	keyFilename := path.Join(path.Dir(certFilename), keyName)

	if _, exists := dirFileSet[keyName]; !exists {
		return fmt.Errorf("missing key %s for certificate %s", keyName, certName)
	}

	cert, err := tls.LoadX509KeyPair(certFilename, keyFilename)
	if err != nil {
		return err
	}

	*certs = append(*certs, cert)

	return nil
}

func prepareCerts(certificateDirnames []string) (pool *x509.CertPool, certs []tls.Certificate) {
	var (
		dirEntries []os.FileInfo
		err        error
	)

	for _, certificateDirname := range certificateDirnames {
		log.Debugf("loading certificates from directory: %s", certificateDirname)

		if dirEntries, err = ioutil.ReadDir(certificateDirname); err != nil {
			log.Errorf("unable to read certificate dir %s: %s", certificateDirname, err)
			continue
		}

		// Make a set of all filenames in the certificate directory.
		dirFileSet := make(map[string]struct{}, len(dirEntries))
		for _, entry := range dirEntries {
			if !entry.IsDir() {
				dirFileSet[entry.Name()] = struct{}{}
			}
		}

		for _, entry := range dirEntries {
			if entry.IsDir() {
				// Only interested in certificate files.
				continue
			}

			pemFilename := path.Join(certificateDirname, entry.Name())

			// CA certs should have a ".crt" suffix while client certs should have
			// a ".cert" suffix. Client keys should have ".key" suffix as well.
			switch {
			case strings.HasSuffix(entry.Name(), ".crt"):
				if pool == nil {
					// It's important to leave the pool *nil* if there are no
					// custom CA certs as a *nil* pool signals the TLS library
					// to load the system certs instead.
					pool = x509.NewCertPool()
				}
				err = addCACertToPool(pemFilename, pool)
			case strings.HasSuffix(entry.Name(), ".cert"):
				err = addClientCert(pemFilename, dirFileSet, &certs)
			}

			if err != nil {
				log.Error(err)
			}
		}
	}

	return
}
