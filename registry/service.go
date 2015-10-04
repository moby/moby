package registry

import (
	"crypto/tls"
	"net/http"
	"net/url"

	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/docker/cliconfig"
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
// and returns OK if authentication was successful.
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

	endpointVersion := APIVersion(APIVersionUnknown)
	if V2Only {
		// Override the endpoint to only attempt a v2 ping
		endpointVersion = APIVersion2
	}

	endpoint, err := NewEndpoint(index, nil, endpointVersion)
	if err != nil {
		return "", err
	}
	authConfig.ServerAddress = endpoint.String()
	return Login(authConfig, endpoint)
}

// Search queries the public registry for images matching the specified
// search terms, and returns the results.
func (s *Service) Search(term string, authConfig *cliconfig.AuthConfig, headers map[string][]string) (*SearchResults, error) {

	repoInfo, err := s.ResolveRepositoryBySearch(term)
	if err != nil {
		return nil, err
	}

	// *TODO: Search multiple indexes.
	endpoint, err := NewEndpoint(repoInfo.Index, http.Header(headers), APIVersionUnknown)
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
	return s.Config.NewRepositoryInfo(name, false)
}

// ResolveRepositoryBySearch splits a repository name into its components
// and configuration of the associated registry.
func (s *Service) ResolveRepositoryBySearch(name string) (*RepositoryInfo, error) {
	return s.Config.NewRepositoryInfo(name, true)
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
	return s.lookupEndpoints(repoName)
}

// LookupPushEndpoints creates an list of endpoints to try to push to, in order of preference.
// It gives preference to v2 endpoints over v1, and HTTPS over plain HTTP.
// Mirrors are not included.
func (s *Service) LookupPushEndpoints(repoName string) (endpoints []APIEndpoint, err error) {
	allEndpoints, err := s.lookupEndpoints(repoName)
	if err == nil {
		for _, endpoint := range allEndpoints {
			if !endpoint.Mirror {
				endpoints = append(endpoints, endpoint)
			}
		}
	}
	return endpoints, err
}

func (s *Service) lookupEndpoints(repoName string) (endpoints []APIEndpoint, err error) {
	endpoints, err = s.lookupV2Endpoints(repoName)
	if err != nil {
		return nil, err
	}

	if V2Only {
		return endpoints, nil
	}

	legacyEndpoints, err := s.lookupV1Endpoints(repoName)
	if err != nil {
		return nil, err
	}
	endpoints = append(endpoints, legacyEndpoints...)

	return endpoints, nil
}
