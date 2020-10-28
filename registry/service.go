package registry // import "github.com/docker/docker/registry"

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultSearchLimit is the default value for maximum number of returned search results.
	DefaultSearchLimit = 25
)

// Service is the interface defining what a registry service should implement.
type Service interface {
	Auth(ctx context.Context, authConfig *types.AuthConfig, userAgent string) (status, token string, err error)
	LookupPullEndpoints(hostname string) (endpoints []APIEndpoint, err error)
	LookupPushEndpoints(hostname string) (endpoints []APIEndpoint, err error)
	ResolveRepository(name reference.Named) (*RepositoryInfo, error)
	Search(ctx context.Context, term string, limit int, authConfig *types.AuthConfig, userAgent string, headers map[string][]string) (*registrytypes.SearchResults, error)
	ServiceConfig() *registrytypes.ServiceConfig
	TLSConfig(hostname string) (*tls.Config, error)
	LoadAllowNondistributableArtifacts([]string) error
	LoadMirrors([]string) error
	LoadInsecureRegistries([]string) error
}

// DefaultService is a registry service. It tracks configuration data such as a list
// of mirrors.
type DefaultService struct {
	config *serviceConfig
	mu     sync.Mutex
}

// NewService returns a new instance of DefaultService ready to be
// installed into an engine.
func NewService(options ServiceOptions) (*DefaultService, error) {
	config, err := newServiceConfig(options)

	return &DefaultService{config: config}, err
}

// ServiceConfig returns the public registry service configuration.
func (s *DefaultService) ServiceConfig() *registrytypes.ServiceConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	servConfig := registrytypes.ServiceConfig{
		AllowNondistributableArtifactsCIDRs:     make([]*(registrytypes.NetIPNet), 0),
		AllowNondistributableArtifactsHostnames: make([]string, 0),
		InsecureRegistryCIDRs:                   make([]*(registrytypes.NetIPNet), 0),
		IndexConfigs:                            make(map[string]*(registrytypes.IndexInfo)),
		Mirrors:                                 make([]string, 0),
	}

	// construct a new ServiceConfig which will not retrieve s.Config directly,
	// and look up items in s.config with mu locked
	servConfig.AllowNondistributableArtifactsCIDRs = append(servConfig.AllowNondistributableArtifactsCIDRs, s.config.ServiceConfig.AllowNondistributableArtifactsCIDRs...)
	servConfig.AllowNondistributableArtifactsHostnames = append(servConfig.AllowNondistributableArtifactsHostnames, s.config.ServiceConfig.AllowNondistributableArtifactsHostnames...)
	servConfig.InsecureRegistryCIDRs = append(servConfig.InsecureRegistryCIDRs, s.config.ServiceConfig.InsecureRegistryCIDRs...)

	for key, value := range s.config.ServiceConfig.IndexConfigs {
		servConfig.IndexConfigs[key] = value
	}

	servConfig.Mirrors = append(servConfig.Mirrors, s.config.ServiceConfig.Mirrors...)

	return &servConfig
}

// LoadAllowNondistributableArtifacts loads allow-nondistributable-artifacts registries for Service.
func (s *DefaultService) LoadAllowNondistributableArtifacts(registries []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.config.LoadAllowNondistributableArtifacts(registries)
}

// LoadMirrors loads registry mirrors for Service
func (s *DefaultService) LoadMirrors(mirrors []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.config.LoadMirrors(mirrors)
}

// LoadInsecureRegistries loads insecure registries for Service
func (s *DefaultService) LoadInsecureRegistries(registries []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.config.LoadInsecureRegistries(registries)
}

// Auth contacts the public registry with the provided credentials,
// and returns OK if authentication was successful.
// It can be used to verify the validity of a client's credentials.
func (s *DefaultService) Auth(ctx context.Context, authConfig *types.AuthConfig, userAgent string) (status, token string, err error) {
	// TODO Use ctx when searching for repositories
	var registryHostName = IndexHostname

	if authConfig.ServerAddress != "" {
		serverAddress := authConfig.ServerAddress
		if !strings.HasPrefix(serverAddress, "https://") && !strings.HasPrefix(serverAddress, "http://") {
			serverAddress = "https://" + serverAddress
		}
		u, err := url.Parse(serverAddress)
		if err != nil {
			return "", "", errdefs.InvalidParameter(errors.Errorf("unable to parse server address: %v", err))
		}
		registryHostName = u.Host
	}

	// Lookup endpoints for authentication using "LookupPushEndpoints", which
	// excludes mirrors to prevent sending credentials of the upstream registry
	// to a mirror.
	endpoints, err := s.LookupPushEndpoints(registryHostName)
	if err != nil {
		return "", "", errdefs.InvalidParameter(err)
	}

	for _, endpoint := range endpoints {
		status, token, err = loginV2(authConfig, endpoint, userAgent)
		if err == nil {
			return
		}
		if fErr, ok := err.(fallbackError); ok {
			logrus.WithError(fErr.err).Infof("Error logging in to endpoint, trying next endpoint")
			continue
		}

		return "", "", err
	}

	return "", "", err
}

// splitReposSearchTerm breaks a search term into an index name and remote name
func splitReposSearchTerm(reposName string) (string, string) {
	nameParts := strings.SplitN(reposName, "/", 2)
	if len(nameParts) == 1 || (!strings.Contains(nameParts[0], ".") &&
		!strings.Contains(nameParts[0], ":") && nameParts[0] != "localhost") {
		// This is a Docker Hub repository (ex: samalba/hipache or ubuntu),
		// use the default Docker Hub registry (docker.io)
		return IndexName, reposName
	}
	return nameParts[0], nameParts[1]
}

// Search queries the public registry for images matching the specified
// search terms, and returns the results.
func (s *DefaultService) Search(ctx context.Context, term string, limit int, authConfig *types.AuthConfig, userAgent string, headers map[string][]string) (*registrytypes.SearchResults, error) {
	// TODO Use ctx when searching for repositories
	if err := validateNoScheme(term); err != nil {
		return nil, err
	}

	indexName, remoteName := splitReposSearchTerm(term)

	// Search is a long-running operation, just lock s.config to avoid block others.
	s.mu.Lock()
	index, err := newIndexInfo(s.config, indexName)
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}

	// *TODO: Search multiple indexes.
	endpoint, err := NewV1Endpoint(index, userAgent, headers)
	if err != nil {
		return nil, err
	}

	var client *http.Client
	if authConfig != nil && authConfig.IdentityToken != "" && authConfig.Username != "" {
		creds := NewStaticCredentialStore(authConfig)
		scopes := []auth.Scope{
			auth.RegistryScope{
				Name:    "catalog",
				Actions: []string{"search"},
			},
		}

		modifiers := Headers(userAgent, nil)
		v2Client, foundV2, err := v2AuthHTTPClient(endpoint.URL, endpoint.client.Transport, modifiers, creds, scopes)
		if err != nil {
			if fErr, ok := err.(fallbackError); ok {
				logrus.Errorf("Cannot use identity token for search, v2 auth not supported: %v", fErr.err)
			} else {
				return nil, err
			}
		} else if foundV2 {
			// Copy non transport http client features
			v2Client.Timeout = endpoint.client.Timeout
			v2Client.CheckRedirect = endpoint.client.CheckRedirect
			v2Client.Jar = endpoint.client.Jar

			logrus.Debugf("using v2 client for search to %s", endpoint.URL)
			client = v2Client
		}
	}

	if client == nil {
		client = endpoint.client
		if err := authorizeClient(client, authConfig, endpoint); err != nil {
			return nil, err
		}
	}

	r := newSession(client, authConfig, endpoint)

	if index.Official {
		// If pull "library/foo", it's stored locally under "foo"
		remoteName = strings.TrimPrefix(remoteName, "library/")
	}
	return r.SearchRepositories(remoteName, limit)
}

// ResolveRepository splits a repository name into its components
// and configuration of the associated registry.
func (s *DefaultService) ResolveRepository(name reference.Named) (*RepositoryInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return newRepositoryInfo(s.config, name)
}

// APIEndpoint represents a remote API endpoint
type APIEndpoint struct {
	Mirror                         bool
	URL                            *url.URL
	Version                        APIVersion
	AllowNondistributableArtifacts bool
	Official                       bool
	TrimHostname                   bool
	TLSConfig                      *tls.Config
}

// ToV1Endpoint returns a V1 API endpoint based on the APIEndpoint
// Deprecated: this function is deprecated and will be removed in a future update
func (e APIEndpoint) ToV1Endpoint(userAgent string, metaHeaders http.Header) *V1Endpoint {
	return newV1Endpoint(*e.URL, e.TLSConfig, userAgent, metaHeaders)
}

// TLSConfig constructs a client TLS configuration based on server defaults
func (s *DefaultService) TLSConfig(hostname string) (*tls.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return newTLSConfig(hostname, isSecureIndex(s.config, hostname))
}

// tlsConfig constructs a client TLS configuration based on server defaults
func (s *DefaultService) tlsConfig(hostname string) (*tls.Config, error) {
	return newTLSConfig(hostname, isSecureIndex(s.config, hostname))
}

func (s *DefaultService) tlsConfigForMirror(mirrorURL *url.URL) (*tls.Config, error) {
	return s.tlsConfig(mirrorURL.Host)
}

// LookupPullEndpoints creates a list of v2 endpoints to try to pull from, in order of preference.
// It gives preference to mirrors over the actual registry, and HTTPS over plain HTTP.
func (s *DefaultService) LookupPullEndpoints(hostname string) (endpoints []APIEndpoint, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.lookupV2Endpoints(hostname)
}

// LookupPushEndpoints creates a list of v2 endpoints to try to push to, in order of preference.
// It gives preference to HTTPS over plain HTTP. Mirrors are not included.
func (s *DefaultService) LookupPushEndpoints(hostname string) (endpoints []APIEndpoint, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	allEndpoints, err := s.lookupV2Endpoints(hostname)
	if err == nil {
		for _, endpoint := range allEndpoints {
			if !endpoint.Mirror {
				endpoints = append(endpoints, endpoint)
			}
		}
	}
	return endpoints, err
}
