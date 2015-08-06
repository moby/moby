package registry

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/tlsconfig"
)

// Service is a registry service. It tracks configuration data such as a list
// of mirrors.
type Service struct {
	Config *ServiceConfig
}

// NewService returns a new instance of Service ready to be
// installed into an engine.
func NewService(options *Options) *Service {
	return &Service{
		Config: NewServiceConfig(options),
	}
}

// Auth contacts the public registry with the provided credentials,
// and returns OK if authentication was sucessful.
// It can be used to verify the validity of a client's credentials.
func (s *Service) Auth(authConfig *cliconfig.AuthConfig) (string, error) {
	addr := authConfig.ServerAddress
	if addr == "" {
		// Use the official registry address if not specified.
		addr = IndexServer
	}
	index, err := s.ResolveIndex(addr)
	if err != nil {
		return "", err
	}
	endpoint, err := NewEndpoint(index, nil)
	if err != nil {
		return "", err
	}
	authConfig.ServerAddress = endpoint.String()
	return Login(authConfig, endpoint)
}

// Search queries the public registry for images matching the specified
// search terms, and returns the results.
func (s *Service) Search(term string, authConfig *cliconfig.AuthConfig, headers map[string][]string) (*SearchResults, error) {
	repoInfo, err := s.ResolveRepository(term)
	if err != nil {
		return nil, err
	}

	// *TODO: Search multiple indexes.
	endpoint, err := repoInfo.GetEndpoint(http.Header(headers))
	if err != nil {
		return nil, err
	}
	r, err := NewSession(endpoint.client, authConfig, endpoint)
	if err != nil {
		return nil, err
	}
	return r.SearchRepositories(repoInfo.GetSearchTerm())
}

// ResolveRepository splits a repository name into its components
// and configuration of the associated registry.
func (s *Service) ResolveRepository(name string) (*RepositoryInfo, error) {
	return s.Config.NewRepositoryInfo(name)
}

// ResolveIndex takes indexName and returns index info
func (s *Service) ResolveIndex(name string) (*IndexInfo, error) {
	return s.Config.NewIndexInfo(name)
}

// APIEndpoint represents a remote API endpoint
type APIEndpoint struct {
	Mirror        bool
	URL           string
	Version       APIVersion
	Official      bool
	TrimHostname  bool
	TLSConfig     *tls.Config
	VersionHeader string
	Versions      []auth.APIVersion
}

// ToV1Endpoint returns a V1 API endpoint based on the APIEndpoint
func (e APIEndpoint) ToV1Endpoint(metaHeaders http.Header) (*Endpoint, error) {
	return newEndpoint(e.URL, e.TLSConfig, metaHeaders)
}

// TLSConfig constructs a client TLS configuration based on server defaults
func (s *Service) TLSConfig(hostname string) (*tls.Config, error) {
	return newTLSConfig(hostname, s.Config.isSecureIndex(hostname))
}

func (s *Service) tlsConfigForMirror(mirror string) (*tls.Config, error) {
	mirrorURL, err := url.Parse(mirror)
	if err != nil {
		return nil, err
	}
	return s.TLSConfig(mirrorURL.Host)
}

// LookupPullEndpoints creates an list of endpoints to try to pull from, in order of preference.
// It gives preference to v2 endpoints over v1, mirrors over the actual
// registry, and HTTPS over plain HTTP.
func (s *Service) LookupPullEndpoints(repoName string) (endpoints []APIEndpoint, err error) {
	return s.lookupEndpoints(repoName, false)
}

// LookupPushEndpoints creates an list of endpoints to try to push to, in order of preference.
// It gives preference to v2 endpoints over v1, and HTTPS over plain HTTP.
// Mirrors are not included.
func (s *Service) LookupPushEndpoints(repoName string) (endpoints []APIEndpoint, err error) {
	return s.lookupEndpoints(repoName, true)
}

func (s *Service) lookupEndpoints(repoName string, isPush bool) (endpoints []APIEndpoint, err error) {
	var cfg = tlsconfig.ServerDefault
	tlsConfig := &cfg
	if strings.HasPrefix(repoName, DefaultNamespace+"/") {
		if !isPush {
			// v2 mirrors for pull only
			for _, mirror := range s.Config.Mirrors {
				mirrorTLSConfig, err := s.tlsConfigForMirror(mirror)
				if err != nil {
					return nil, err
				}
				endpoints = append(endpoints, APIEndpoint{
					URL: mirror,
					// guess mirrors are v2
					Version:      APIVersion2,
					Mirror:       true,
					TrimHostname: true,
					TLSConfig:    mirrorTLSConfig,
				})
			}
		}
		// v2 registry
		endpoints = append(endpoints, APIEndpoint{
			URL:          DefaultV2Registry,
			Version:      APIVersion2,
			Official:     true,
			TrimHostname: true,
			TLSConfig:    tlsConfig,
		})
		if runtime.GOOS == "linux" { // do not inherit legacy API for OSes supported in the future
			// v1 registry
			endpoints = append(endpoints, APIEndpoint{
				URL:          DefaultV1Registry,
				Version:      APIVersion1,
				Official:     true,
				TrimHostname: true,
				TLSConfig:    tlsConfig,
			})
		}
		return endpoints, nil
	}

	slashIndex := strings.IndexRune(repoName, '/')
	if slashIndex <= 0 {
		return nil, fmt.Errorf("invalid repo name: missing '/':  %s", repoName)
	}
	hostname := repoName[:slashIndex]

	tlsConfig, err = s.TLSConfig(hostname)
	if err != nil {
		return nil, err
	}
	isSecure := !tlsConfig.InsecureSkipVerify

	v2Versions := []auth.APIVersion{
		{
			Type:    "registry",
			Version: "2.0",
		},
	}
	endpoints = []APIEndpoint{
		{
			URL:           "https://" + hostname,
			Version:       APIVersion2,
			TrimHostname:  true,
			TLSConfig:     tlsConfig,
			VersionHeader: DefaultRegistryVersionHeader,
			Versions:      v2Versions,
		},
		{
			URL:          "https://" + hostname,
			Version:      APIVersion1,
			TrimHostname: true,
			TLSConfig:    tlsConfig,
		},
	}

	if !isSecure {
		endpoints = append(endpoints, APIEndpoint{
			URL:          "http://" + hostname,
			Version:      APIVersion2,
			TrimHostname: true,
			// used to check if supposed to be secure via InsecureSkipVerify
			TLSConfig:     tlsConfig,
			VersionHeader: DefaultRegistryVersionHeader,
			Versions:      v2Versions,
		}, APIEndpoint{
			URL:          "http://" + hostname,
			Version:      APIVersion1,
			TrimHostname: true,
			// used to check if supposed to be secure via InsecureSkipVerify
			TLSConfig: tlsConfig,
		})
	}

	return endpoints, nil
}
