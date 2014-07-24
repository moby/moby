package registry

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
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

	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/utils"
)

var (
	ErrAlreadyExists         = errors.New("Image already exists")
	ErrInvalidRepositoryName = errors.New("Invalid repository name (ex: \"registry.domain.tld/myrepos\")")
	errLoginRequired         = errors.New("Authentication is required.")
)

type TimeoutType uint32

const (
	NoTimeout TimeoutType = iota
	ReceiveTimeout
	ConnectTimeout
)

func newClient(jar http.CookieJar, roots *x509.CertPool, cert *tls.Certificate, timeout TimeoutType) *http.Client {
	tlsConfig := tls.Config{RootCAs: roots}

	if cert != nil {
		tlsConfig.Certificates = append(tlsConfig.Certificates, *cert)
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
			conn, err := net.DialTimeout(proto, addr, 5*time.Second)
			if err != nil {
				return nil, err
			}
			// Set the recv timeout to 10 seconds
			conn.SetDeadline(time.Now().Add(10 * time.Second))
			return conn, nil
		}
	case ReceiveTimeout:
		httpTransport.Dial = func(proto string, addr string) (net.Conn, error) {
			conn, err := net.Dial(proto, addr)
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

func doRequest(req *http.Request, jar http.CookieJar, timeout TimeoutType) (*http.Response, *http.Client, error) {
	hasFile := func(files []os.FileInfo, name string) bool {
		for _, f := range files {
			if f.Name() == name {
				return true
			}
		}
		return false
	}

	hostDir := path.Join("/etc/docker/certs.d", req.URL.Host)
	fs, err := ioutil.ReadDir(hostDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}

	var (
		pool  *x509.CertPool
		certs []*tls.Certificate
	)

	for _, f := range fs {
		if strings.HasSuffix(f.Name(), ".crt") {
			if pool == nil {
				pool = x509.NewCertPool()
			}
			data, err := ioutil.ReadFile(path.Join(hostDir, f.Name()))
			if err != nil {
				return nil, nil, err
			} else {
				pool.AppendCertsFromPEM(data)
			}
		}
		if strings.HasSuffix(f.Name(), ".cert") {
			certName := f.Name()
			keyName := certName[:len(certName)-5] + ".key"
			if !hasFile(fs, keyName) {
				return nil, nil, fmt.Errorf("Missing key %s for certificate %s", keyName, certName)
			} else {
				cert, err := tls.LoadX509KeyPair(path.Join(hostDir, certName), path.Join(hostDir, keyName))
				if err != nil {
					return nil, nil, err
				}
				certs = append(certs, &cert)
			}
		}
		if strings.HasSuffix(f.Name(), ".key") {
			keyName := f.Name()
			certName := keyName[:len(keyName)-4] + ".cert"
			if !hasFile(fs, certName) {
				return nil, nil, fmt.Errorf("Missing certificate %s for key %s", certName, keyName)
			}
		}
	}

	if len(certs) == 0 {
		client := newClient(jar, pool, nil, timeout)
		res, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		return res, client, nil
	} else {
		for i, cert := range certs {
			client := newClient(jar, pool, cert, timeout)
			res, err := client.Do(req)
			if i == len(certs)-1 {
				// If this is the last cert, always return the result
				return res, client, err
			} else {
				// Otherwise, continue to next cert if 403 or 5xx
				if err == nil && res.StatusCode != 403 && !(res.StatusCode >= 500 && res.StatusCode < 600) {
					return res, client, err
				}
			}
		}
	}

	return nil, nil, nil
}

func pingRegistryEndpoint(endpoint string) (RegistryInfo, error) {
	if endpoint == IndexServerAddress() {
		// Skip the check, we now this one is valid
		// (and we never want to fallback to http in case of error)
		return RegistryInfo{Standalone: false}, nil
	}

	req, err := http.NewRequest("GET", endpoint+"_ping", nil)
	if err != nil {
		return RegistryInfo{Standalone: false}, err
	}

	resp, _, err := doRequest(req, nil, ConnectTimeout)
	if err != nil {
		return RegistryInfo{Standalone: false}, err
	}

	defer resp.Body.Close()

	jsonString, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return RegistryInfo{Standalone: false}, fmt.Errorf("Error while reading the http response: %s", err)
	}

	// If the header is absent, we assume true for compatibility with earlier
	// versions of the registry. default to true
	info := RegistryInfo{
		Standalone: true,
	}
	if err := json.Unmarshal(jsonString, &info); err != nil {
		log.Debugf("Error unmarshalling the _ping RegistryInfo: %s", err)
		// don't stop here. Just assume sane defaults
	}
	if hdr := resp.Header.Get("X-Docker-Registry-Version"); hdr != "" {
		log.Debugf("Registry version header: '%s'", hdr)
		info.Version = hdr
	}
	log.Debugf("RegistryInfo.Version: %q", info.Version)

	standalone := resp.Header.Get("X-Docker-Registry-Standalone")
	log.Debugf("Registry standalone header: '%s'", standalone)
	// Accepted values are "true" (case-insensitive) and "1".
	if strings.EqualFold(standalone, "true") || standalone == "1" {
		info.Standalone = true
	} else if len(standalone) > 0 {
		// there is a header set, and it is not "true" or "1", so assume fails
		info.Standalone = false
	}
	log.Debugf("RegistryInfo.Standalone: %q", info.Standalone)
	return info, nil
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
	} else {
		namespace = nameParts[0]
		name = nameParts[1]
	}
	validNamespace := regexp.MustCompile(`^([a-z0-9_]{4,30})$`)
	if !validNamespace.MatchString(namespace) {
		return fmt.Errorf("Invalid namespace name (%s), only [a-z0-9_] are allowed, size between 4 and 30", namespace)
	}
	validRepo := regexp.MustCompile(`^([a-z0-9-_.]+)$`)
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

// this method expands the registry name as used in the prefix of a repo
// to a full url. if it already is a url, there will be no change.
// The registry is pinged to test if it http or https
func ExpandAndVerifyRegistryUrl(hostname string) (string, error) {
	if strings.HasPrefix(hostname, "http:") || strings.HasPrefix(hostname, "https:") {
		// if there is no slash after https:// (8 characters) then we have no path in the url
		if strings.LastIndex(hostname, "/") < 9 {
			// there is no path given. Expand with default path
			hostname = hostname + "/v1/"
		}
		if _, err := pingRegistryEndpoint(hostname); err != nil {
			return "", errors.New("Invalid Registry endpoint: " + err.Error())
		}
		return hostname, nil
	}
	endpoint := fmt.Sprintf("https://%s/v1/", hostname)
	if _, err := pingRegistryEndpoint(endpoint); err != nil {
		log.Debugf("Registry %s does not work (%s), falling back to http", endpoint, err)
		endpoint = fmt.Sprintf("http://%s/v1/", hostname)
		if _, err = pingRegistryEndpoint(endpoint); err != nil {
			//TODO: triggering highland build can be done there without "failing"
			return "", errors.New("Invalid Registry endpoint: " + err.Error())
		}
	}
	return endpoint, nil
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
		} else {
			for k, v := range via[0].Header {
				if k != "Authorization" {
					for _, vv := range v {
						req.Header.Add(k, vv)
					}
				}
			}
		}
	}
	return nil
}
