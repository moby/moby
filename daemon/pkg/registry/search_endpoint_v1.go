package registry

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types/registry"
)

// v1PingResult contains the information returned when pinging a registry. It
// indicates whether the registry claims to be a standalone registry.
type v1PingResult struct {
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
func newV1Endpoint(ctx context.Context, index *registry.IndexInfo, headers http.Header) (*v1Endpoint, error) {
	tlsConfig, err := newTLSConfig(ctx, index.Name, index.Secure)
	if err != nil {
		return nil, err
	}

	endpoint, err := newV1EndpointFromStr(GetAuthConfigKey(index), tlsConfig, headers)
	if err != nil {
		return nil, err
	}

	if endpoint.String() == IndexServer {
		// Skip the check, we know this one is valid
		// (and we never want to fall back to http in case of error)
		return endpoint, nil
	}

	// Try HTTPS ping to registry
	endpoint.URL.Scheme = "https"
	if _, err := endpoint.ping(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if endpoint.IsSecure {
			// If registry is secure and HTTPS failed, show user the error and tell them about `--insecure-registry`
			// in case that's what they need. DO NOT accept unknown CA certificates, and DO NOT fall back to HTTP.
			return nil, invalidParamf("invalid registry endpoint %s: %v. If this private registry supports only HTTP or HTTPS with an unknown CA certificate, please add `--insecure-registry %s` to the daemon's arguments. In the case of HTTPS, if you have access to the registry's CA certificate, no need for the flag; simply place the CA certificate at /etc/docker/certs.d/%s/ca.crt", endpoint, err, endpoint.URL.Host, endpoint.URL.Host)
		}

		// registry is insecure and HTTPS failed, fallback to HTTP.
		log.G(ctx).WithError(err).Debugf("error from registry %q marked as insecure - insecurely falling back to HTTP", endpoint)
		endpoint.URL.Scheme = "http"
		if _, err2 := endpoint.ping(ctx); err2 != nil {
			return nil, invalidParamf("invalid registry endpoint %q. HTTPS attempt: %v. HTTP attempt: %v", endpoint, err, err2)
		}
	}

	return endpoint, nil
}

// trimV1Address trims the "v1" version suffix off the address and returns
// the trimmed address. It returns an error on "v2" endpoints.
func trimV1Address(address string) (string, error) {
	trimmed := strings.TrimSuffix(address, "/")
	if strings.HasSuffix(trimmed, "/v2") {
		return "", invalidParamf("search is not supported on v2 endpoints: %s", address)
	}
	return strings.TrimSuffix(trimmed, "/v1"), nil
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
func (e *v1Endpoint) ping(ctx context.Context) (v1PingResult, error) {
	if e.String() == IndexServer {
		// Skip the check, we know this one is valid
		// (and we never want to fallback to http in case of error)
		return v1PingResult{}, nil
	}

	pingURL := e.String() + "_ping"
	log.G(ctx).WithField("url", pingURL).Debug("attempting v1 ping for registry endpoint")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, http.NoBody)
	if err != nil {
		return v1PingResult{}, invalidParam(err)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return v1PingResult{}, err
		}
		return v1PingResult{}, invalidParam(err)
	}

	defer resp.Body.Close()

	if v := resp.Header.Get("X-Docker-Registry-Standalone"); v != "" {
		info := v1PingResult{}
		// Accepted values are "1", and "true" (case-insensitive).
		if v == "1" || strings.EqualFold(v, "true") {
			info.Standalone = true
		}
		log.G(ctx).Debugf("v1PingResult.Standalone (from X-Docker-Registry-Standalone header): %t", info.Standalone)
		return info, nil
	}

	// If the header is absent, we assume true for compatibility with earlier
	// versions of the registry. default to true
	info := v1PingResult{
		Standalone: true,
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		log.G(ctx).WithError(err).Debug("error unmarshaling _ping response")
		// don't stop here. Just assume sane defaults
	}

	log.G(ctx).Debugf("v1PingResult.Standalone: %t", info.Standalone)
	return info, nil
}

// httpClient returns an HTTP client structure which uses the given transport
// and contains the necessary headers for redirected requests
func httpClient(transport http.RoundTripper) *http.Client {
	return &http.Client{
		Transport:     transport,
		CheckRedirect: addRequiredHeadersToRedirectedRequests,
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

// addRequiredHeadersToRedirectedRequests adds the necessary redirection headers
// for redirected requests
func addRequiredHeadersToRedirectedRequests(req *http.Request, via []*http.Request) error {
	if len(via) != 0 && via[0] != nil {
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
