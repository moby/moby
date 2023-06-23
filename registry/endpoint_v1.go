package registry // import "github.com/docker/docker/registry"

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/containerd/containerd/log"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types/registry"
)

// v1PingResult contains the information returned when pinging a registry. It
// indicates the registry's version and whether the registry claims to be a
// standalone registry.
type v1PingResult struct {
	// Version is the registry version supplied by the registry in an HTTP
	// header
	Version string `json:"version"`
	// Standalone is set to true if the registry indicates it is a
	// standalone registry in the X-Docker-Registry-Standalone
	// header
	Standalone bool `json:"standalone"`
}

// v1Endpoint stores basic information about a V1 registry endpoint.
type v1Endpoint struct {
	client   *http.Client
	URL      *url.URL
	IsSecure bool
}

// newV1Endpoint parses the given address to return a registry endpoint.
// TODO: remove. This is only used by search.
func newV1Endpoint(index *registry.IndexInfo, headers http.Header) (*v1Endpoint, error) {
	tlsConfig, err := newTLSConfig(index.Name, index.Secure)
	if err != nil {
		return nil, err
	}

	endpoint, err := newV1EndpointFromStr(GetAuthConfigKey(index), tlsConfig, headers)
	if err != nil {
		return nil, err
	}

	err = validateEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	return endpoint, nil
}

func validateEndpoint(endpoint *v1Endpoint) error {
	log.G(context.TODO()).Debugf("pinging registry endpoint %s", endpoint)

	// Try HTTPS ping to registry
	endpoint.URL.Scheme = "https"
	if _, err := endpoint.ping(); err != nil {
		if endpoint.IsSecure {
			// If registry is secure and HTTPS failed, show user the error and tell them about `--insecure-registry`
			// in case that's what they need. DO NOT accept unknown CA certificates, and DO NOT fallback to HTTP.
			return invalidParamf("invalid registry endpoint %s: %v. If this private registry supports only HTTP or HTTPS with an unknown CA certificate, please add `--insecure-registry %s` to the daemon's arguments. In the case of HTTPS, if you have access to the registry's CA certificate, no need for the flag; simply place the CA certificate at /etc/docker/certs.d/%s/ca.crt", endpoint, err, endpoint.URL.Host, endpoint.URL.Host)
		}

		// If registry is insecure and HTTPS failed, fallback to HTTP.
		log.G(context.TODO()).WithError(err).Debugf("error from registry %q marked as insecure - insecurely falling back to HTTP", endpoint)
		endpoint.URL.Scheme = "http"

		var err2 error
		if _, err2 = endpoint.ping(); err2 == nil {
			return nil
		}

		return invalidParamf("invalid registry endpoint %q. HTTPS attempt: %v. HTTP attempt: %v", endpoint, err, err2)
	}

	return nil
}

// trimV1Address trims the version off the address and returns the
// trimmed address or an error if there is a non-V1 version.
func trimV1Address(address string) (string, error) {
	address = strings.TrimSuffix(address, "/")
	chunks := strings.Split(address, "/")
	apiVersionStr := chunks[len(chunks)-1]
	if apiVersionStr == "v1" {
		return strings.Join(chunks[:len(chunks)-1], "/"), nil
	}

	for k, v := range apiVersions {
		if k != APIVersion1 && apiVersionStr == v {
			return "", invalidParamf("unsupported V1 version path %s", apiVersionStr)
		}
	}

	return address, nil
}

func newV1EndpointFromStr(address string, tlsConfig *tls.Config, headers http.Header) (*v1Endpoint, error) {
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "https://" + address
	}

	address, err := trimV1Address(address)
	if err != nil {
		return nil, err
	}

	uri, err := url.Parse(address)
	if err != nil {
		return nil, invalidParam(err)
	}

	// TODO(tiborvass): make sure a ConnectTimeout transport is used
	tr := newTransport(tlsConfig)

	return &v1Endpoint{
		IsSecure: tlsConfig == nil || !tlsConfig.InsecureSkipVerify,
		URL:      uri,
		client:   httpClient(transport.NewTransport(tr, Headers("", headers)...)),
	}, nil
}

// Get the formatted URL for the root of this registry Endpoint
func (e *v1Endpoint) String() string {
	return e.URL.String() + "/v1/"
}

// ping returns a v1PingResult which indicates whether the registry is standalone or not.
func (e *v1Endpoint) ping() (v1PingResult, error) {
	if e.String() == IndexServer {
		// Skip the check, we know this one is valid
		// (and we never want to fallback to http in case of error)
		return v1PingResult{}, nil
	}

	log.G(context.TODO()).Debugf("attempting v1 ping for registry endpoint %s", e)
	pingURL := e.String() + "_ping"
	req, err := http.NewRequest(http.MethodGet, pingURL, nil)
	if err != nil {
		return v1PingResult{}, invalidParam(err)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return v1PingResult{}, invalidParam(err)
	}

	defer resp.Body.Close()

	jsonString, err := io.ReadAll(resp.Body)
	if err != nil {
		return v1PingResult{}, invalidParamWrapf(err, "error while reading response from %s", pingURL)
	}

	// If the header is absent, we assume true for compatibility with earlier
	// versions of the registry. default to true
	info := v1PingResult{
		Standalone: true,
	}
	if err := json.Unmarshal(jsonString, &info); err != nil {
		log.G(context.TODO()).WithError(err).Debug("error unmarshaling _ping response")
		// don't stop here. Just assume sane defaults
	}
	if hdr := resp.Header.Get("X-Docker-Registry-Version"); hdr != "" {
		info.Version = hdr
	}
	log.G(context.TODO()).Debugf("v1PingResult.Version: %q", info.Version)

	standalone := resp.Header.Get("X-Docker-Registry-Standalone")

	// Accepted values are "true" (case-insensitive) and "1".
	if strings.EqualFold(standalone, "true") || standalone == "1" {
		info.Standalone = true
	} else if len(standalone) > 0 {
		// there is a header set, and it is not "true" or "1", so assume fails
		info.Standalone = false
	}
	log.G(context.TODO()).Debugf("v1PingResult.Standalone: %t", info.Standalone)
	return info, nil
}
