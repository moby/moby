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
	validNamespaceChars      = regexp.MustCompile(`^([a-z0-9-_]*)$`)
	validRepo                = regexp.MustCompile(`^([a-z0-9-_.]+)$`)
	emptyServiceConfig       = NewServiceConfig(nil)
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

func validateRemoteName(remoteName string) error {
	var (
		namespace string
		name      string
	)
	nameParts := strings.SplitN(remoteName, "/", 2)
	if len(nameParts) < 2 {
		namespace = "library"
		name = nameParts[0]

		// the repository name must not be a valid image ID
		if err := utils.ValidateID(name); err == nil {
			return fmt.Errorf("Invalid repository name (%s), cannot specify 64-byte hexadecimal strings", name)
		}
	} else {
		namespace = nameParts[0]
		name = nameParts[1]
	}
	if !validNamespaceChars.MatchString(namespace) {
		return fmt.Errorf("Invalid namespace name (%s). Only [a-z0-9-_] are allowed.", namespace)
	}
	if len(namespace) < 4 || len(namespace) > 30 {
		return fmt.Errorf("Invalid namespace name (%s). Cannot be fewer than 4 or more than 30 characters.", namespace)
	}
	if strings.HasPrefix(namespace, "-") || strings.HasSuffix(namespace, "-") {
		return fmt.Errorf("Invalid namespace name (%s). Cannot begin or end with a hyphen.", namespace)
	}
	if strings.Contains(namespace, "--") {
		return fmt.Errorf("Invalid namespace name (%s). Cannot contain consecutive hyphens.", namespace)
	}
	if !validRepo.MatchString(name) {
		return fmt.Errorf("Invalid repository name (%s), only [a-z0-9-_.] are allowed", name)
	}
	return nil
}

// NewIndexInfo returns IndexInfo configuration from indexName
func NewIndexInfo(config *ServiceConfig, indexName string) (*IndexInfo, error) {
	var err error
	indexName, err = ValidateIndexName(indexName)
	if err != nil {
		return nil, err
	}

	// Return any configured index info, first.
	if index, ok := config.IndexConfigs[indexName]; ok {
		return index, nil
	}

	// Construct a non-configured index info.
	index := &IndexInfo{
		Name:     indexName,
		Mirrors:  make([]string, 0),
		Official: false,
	}
	index.Secure = config.isSecureIndex(indexName)
	return index, nil
}

func validateNoSchema(reposName string) error {
	if strings.Contains(reposName, "://") {
		// It cannot contain a scheme!
		return ErrInvalidRepositoryName
	}
	return nil
}

// splitReposName breaks a reposName into an index name and remote name
func splitReposName(reposName string) (string, string) {
	nameParts := strings.SplitN(reposName, "/", 2)
	var indexName, remoteName string
	if len(nameParts) == 1 || (!strings.Contains(nameParts[0], ".") &&
		!strings.Contains(nameParts[0], ":") && nameParts[0] != "localhost") {
		// This is a Docker Index repos (ex: samalba/hipache or ubuntu)
		// 'docker.io'
		indexName = IndexServerName()
		remoteName = reposName
	} else {
		indexName = nameParts[0]
		remoteName = nameParts[1]
	}
	return indexName, remoteName
}

// NewRepositoryInfo validates and breaks down a repository name into a RepositoryInfo
func NewRepositoryInfo(config *ServiceConfig, reposName string) (*RepositoryInfo, error) {
	if err := validateNoSchema(reposName); err != nil {
		return nil, err
	}

	indexName, remoteName := splitReposName(reposName)
	if err := validateRemoteName(remoteName); err != nil {
		return nil, err
	}

	repoInfo := &RepositoryInfo{
		RemoteName: remoteName,
	}

	var err error
	repoInfo.Index, err = NewIndexInfo(config, indexName)
	if err != nil {
		return nil, err
	}

	if repoInfo.Index.Official {
		normalizedName := repoInfo.RemoteName
		if strings.HasPrefix(normalizedName, "library/") {
			// If pull "library/foo", it's stored locally under "foo"
			normalizedName = strings.SplitN(normalizedName, "/", 2)[1]
		}

		repoInfo.LocalName = normalizedName
		repoInfo.RemoteName = normalizedName
		// If the normalized name does not contain a '/' (e.g. "foo")
		// then it is an official repo.
		if strings.IndexRune(normalizedName, '/') == -1 {
			repoInfo.Official = true
			// Fix up remote name for official repos.
			repoInfo.RemoteName = "library/" + normalizedName
		}

		// *TODO: Prefix this with 'docker.io/'.
		repoInfo.CanonicalName = repoInfo.LocalName
	} else {
		// *TODO: Decouple index name from hostname (via registry configuration?)
		repoInfo.LocalName = repoInfo.Index.Name + "/" + repoInfo.RemoteName
		repoInfo.CanonicalName = repoInfo.LocalName
	}
	return repoInfo, nil
}

// ValidateRepositoryName validates a repository name
func ValidateRepositoryName(reposName string) error {
	var err error
	if err = validateNoSchema(reposName); err != nil {
		return err
	}
	indexName, remoteName := splitReposName(reposName)
	if _, err = ValidateIndexName(indexName); err != nil {
		return err
	}
	return validateRemoteName(remoteName)
}

// ParseRepositoryInfo performs the breakdown of a repository name into a RepositoryInfo, but
// lacks registry configuration.
func ParseRepositoryInfo(reposName string) (*RepositoryInfo, error) {
	return NewRepositoryInfo(emptyServiceConfig, reposName)
}

// NormalizeLocalName transforms a repository name into a normalize LocalName
// Passes through the name without transformation on error (image id, etc)
func NormalizeLocalName(name string) string {
	repoInfo, err := ParseRepositoryInfo(name)
	if err != nil {
		return name
	}
	return repoInfo.LocalName
}

// GetAuthConfigKey special-cases using the full index address of the official
// index as the AuthConfig key, and uses the (host)name[:port] for private indexes.
func (index *IndexInfo) GetAuthConfigKey() string {
	if index.Official {
		return IndexServerAddress()
	}
	return index.Name
}

// GetSearchTerm special-cases using local name for official index, and
// remote name for private indexes.
func (repoInfo *RepositoryInfo) GetSearchTerm() string {
	if repoInfo.Index.Official {
		return repoInfo.LocalName
	}
	return repoInfo.RemoteName
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
