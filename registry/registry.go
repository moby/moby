package registry

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/utils"
)

var (
	ErrAlreadyExists         = errors.New("Image already exists")
	ErrInvalidRepositoryName = errors.New("Invalid repository name (ex: \"registry.domain.tld/myrepos\")")
	ErrDoesNotExist          = errors.New("Image does not exist")
	errLoginRequired         = errors.New("Authentication is required.")
	validHex                 = regexp.MustCompile(`^([a-f0-9]{64})$`)
	validNamespace           = regexp.MustCompile(`^([a-z0-9_]{4,30})$`)
	validRepo                = regexp.MustCompile(`^([a-z0-9-_.]+)$`)
)

type TimeoutType uint32

const (
	NoTimeout TimeoutType = iota
	ReceiveTimeout
	ConnectTimeout
)

func newClient(jar http.CookieJar, roots *x509.CertPool, certs []tls.Certificate, timeout TimeoutType, secure bool) *http.Client {
	tlsConfig := tls.Config{
		RootCAs: roots,
		// Avoid fallback to SSL protocols < TLS1.0
		MinVersion:   tls.VersionTLS10,
		Certificates: certs,
	}

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

	return &http.Client{
		Transport:     httpTransport,
		CheckRedirect: AddRequiredHeadersToRedirectedRequests,
		Jar:           jar,
	}
}

func doRequest(req *http.Request, jar http.CookieJar, timeout TimeoutType, secure bool) (*http.Response, *http.Client, error) {
	var (
		pool  *x509.CertPool
		certs []tls.Certificate
	)

	if secure && req.URL.Scheme == "https" {
		hasFile := func(files []os.FileInfo, name string) bool {
			for _, f := range files {
				if f.Name() == name {
					return true
				}
			}
			return false
		}

		hostDir := path.Join("/etc/docker/certs.d", req.URL.Host)
		log.Debugf("hostDir: %s", hostDir)
		fs, err := ioutil.ReadDir(hostDir)
		if err != nil && !os.IsNotExist(err) {
			return nil, nil, err
		}

		for _, f := range fs {
			if strings.HasSuffix(f.Name(), ".crt") {
				if pool == nil {
					pool = x509.NewCertPool()
				}
				log.Debugf("crt: %s", hostDir+"/"+f.Name())
				data, err := ioutil.ReadFile(path.Join(hostDir, f.Name()))
				if err != nil {
					return nil, nil, err
				}
				pool.AppendCertsFromPEM(data)
			}
			if strings.HasSuffix(f.Name(), ".cert") {
				certName := f.Name()
				keyName := certName[:len(certName)-5] + ".key"
				log.Debugf("cert: %s", hostDir+"/"+f.Name())
				if !hasFile(fs, keyName) {
					return nil, nil, fmt.Errorf("Missing key %s for certificate %s", keyName, certName)
				}
				cert, err := tls.LoadX509KeyPair(path.Join(hostDir, certName), path.Join(hostDir, keyName))
				if err != nil {
					return nil, nil, err
				}
				certs = append(certs, cert)
			}
			if strings.HasSuffix(f.Name(), ".key") {
				keyName := f.Name()
				certName := keyName[:len(keyName)-4] + ".cert"
				log.Debugf("key: %s", hostDir+"/"+f.Name())
				if !hasFile(fs, certName) {
					return nil, nil, fmt.Errorf("Missing certificate %s for key %s", certName, keyName)
				}
			}
		}
	}

	if len(certs) == 0 {
		client := newClient(jar, pool, nil, timeout, secure)
		res, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		return res, client, nil
	}

	client := newClient(jar, pool, certs, timeout, secure)
	res, err := client.Do(req)
	return res, client, err
}

func validateRepositoryName(repositoryName string) error {
	var (
		namespace string
		name      string
	)
	nameParts := strings.SplitN(repositoryName, "/", 2)
	if len(nameParts) < 2 {
		namespace = "library"
		name = nameParts[0]

		if validHex.MatchString(name) {
			return fmt.Errorf("Invalid repository name (%s), cannot specify 64-byte hexadecimal strings", name)
		}
	} else {
		namespace = nameParts[0]
		name = nameParts[1]
	}
	if !validNamespace.MatchString(namespace) {
		return fmt.Errorf("Invalid namespace name (%s), only [a-z0-9_] are allowed, size between 4 and 30", namespace)
	}
	if !validRepo.MatchString(name) {
		return fmt.Errorf("Invalid repository name (%s), only [a-z0-9-_.] are allowed", name)
	}
	return nil
}

// Resolves a repository name to a hostname + name
func ResolveRepositoryName(reposName string) (string, string, error) {
	if strings.Contains(reposName, "://") {
		// It cannot contain a scheme!
		return "", "", ErrInvalidRepositoryName
	}
	nameParts := strings.SplitN(reposName, "/", 2)
	if len(nameParts) == 1 || (!strings.Contains(nameParts[0], ".") && !strings.Contains(nameParts[0], ":") &&
		nameParts[0] != "localhost") {
		// This is a Docker Index repos (ex: samalba/hipache or ubuntu)
		err := validateRepositoryName(reposName)
		return IndexServerAddress(), reposName, err
	}
	hostname := nameParts[0]
	reposName = nameParts[1]
	if strings.Contains(hostname, "index.docker.io") {
		return "", "", fmt.Errorf("Invalid repository name, try \"%s\" instead", reposName)
	}
	if err := validateRepositoryName(reposName); err != nil {
		return "", "", err
	}

	return hostname, reposName, nil
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
