package registry

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/docker/docker/pkg/log"
)

// scans string for api version in the URL path. returns the trimmed hostname, if version found, string and API version.
func scanForApiVersion(hostname string) (string, APIVersion) {
	var (
		chunks        []string
		apiVersionStr string
	)
	if strings.HasSuffix(hostname, "/") {
		chunks = strings.Split(hostname[:len(hostname)-1], "/")
		apiVersionStr = chunks[len(chunks)-1]
	} else {
		chunks = strings.Split(hostname, "/")
		apiVersionStr = chunks[len(chunks)-1]
	}
	for k, v := range apiVersions {
		if apiVersionStr == v {
			hostname = strings.Join(chunks[:len(chunks)-1], "/")
			return hostname, k
		}
	}
	return hostname, DefaultAPIVersion
}

func NewEndpoint(hostname string, insecureRegistries []string) (*Endpoint, error) {
	endpoint, err := newEndpoint(hostname)
	if err != nil {
		return nil, err
	}

	secure := isSecure(endpoint.URL.Host, insecureRegistries)
	endpoint.secure = secure

	// Try HTTPS ping to registry
	endpoint.URL.Scheme = "https"
	if _, err := endpoint.Ping(); err != nil {

		//TODO: triggering highland build can be done there without "failing"

		if secure {
			// If registry is secure and HTTPS failed, show user the error and tell them about `--insecure-registry`
			// in case that's what they need. DO NOT accept unknown CA certificates, and DO NOT fallback to HTTP.
			return nil, fmt.Errorf("Invalid registry endpoint %s: %v. If this private registry supports only HTTP or HTTPS with an unknown CA certificate, please add `--insecure-registry %s` to the daemon's arguments. In the case of HTTPS, if you have access to the registry's CA certificate, no need for the flag; simply place the CA certificate at /etc/docker/certs.d/%s/ca.crt", endpoint, err, endpoint.URL.Host, endpoint.URL.Host)
		}

		// If registry is insecure and HTTPS failed, fallback to HTTP.
		log.Debugf("Error from registry %q marked as insecure: %v. Insecurely falling back to HTTP", endpoint, err)
		endpoint.URL.Scheme = "http"
		_, err2 := endpoint.Ping()
		if err2 == nil {
			return endpoint, nil
		}

		return nil, fmt.Errorf("Invalid registry endpoint %q. HTTPS attempt: %v. HTTP attempt: %v", endpoint, err, err2)
	}

	return endpoint, nil
}
func newEndpoint(hostname string) (*Endpoint, error) {
	var (
		endpoint        = Endpoint{secure: true}
		trimmedHostname string
		err             error
	)
	if !strings.HasPrefix(hostname, "http") {
		hostname = "https://" + hostname
	}
	trimmedHostname, endpoint.Version = scanForApiVersion(hostname)
	endpoint.URL, err = url.Parse(trimmedHostname)
	if err != nil {
		return nil, err
	}
	return &endpoint, nil
}

type Endpoint struct {
	URL     *url.URL
	Version APIVersion
	secure  bool
}

// Get the formated URL for the root of this registry Endpoint
func (e Endpoint) String() string {
	return fmt.Sprintf("%s/v%d/", e.URL.String(), e.Version)
}

func (e Endpoint) VersionString(version APIVersion) string {
	return fmt.Sprintf("%s/v%d/", e.URL.String(), version)
}

func (e Endpoint) Ping() (RegistryInfo, error) {
	if e.String() == IndexServerAddress() {
		// Skip the check, we now this one is valid
		// (and we never want to fallback to http in case of error)
		return RegistryInfo{Standalone: false}, nil
	}

	req, err := http.NewRequest("GET", e.String()+"_ping", nil)
	if err != nil {
		return RegistryInfo{Standalone: false}, err
	}

	resp, _, err := doRequest(req, nil, ConnectTimeout, e.secure)
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
	log.Debugf("RegistryInfo.Standalone: %t", info.Standalone)
	return info, nil
}

// isSecure returns false if the provided hostname is part of the list of insecure registries.
// Insecure registries accept HTTP and/or accept HTTPS with certificates from unknown CAs.
func isSecure(hostname string, insecureRegistries []string) bool {
	if hostname == IndexServerAddress() {
		return true
	}

	for _, h := range insecureRegistries {
		if hostname == h {
			return false
		}
	}

	return true
}
