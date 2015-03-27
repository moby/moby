package registry

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/reference"
	"github.com/docker/engine-api/types"
	registrytypes "github.com/docker/engine-api/types/registry"
)

// Service is a registry service. It tracks configuration data such as a list
// of mirrors.
type Service struct {
	Config *registrytypes.ServiceConfig
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
func (s *Service) Auth(authConfig *types.AuthConfig) (string, error) {
	addr := authConfig.ServerAddress
	if addr == "" {
		// Use the official registry address if not specified.
		addr = IndexServerAddress()
	}
	if addr == "" {
		return "", fmt.Errorf("No configured registry to authenticate to.")
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

type by func(fst, snd *registrytypes.SearchResultExt) bool

type searchResultSorter struct {
	Results []registrytypes.SearchResultExt
	By      func(fst, snd *registrytypes.SearchResultExt) bool
}

func (by by) Sort(results []registrytypes.SearchResultExt) {
	rs := &searchResultSorter{
		Results: results,
		By:      by,
	}
	sort.Sort(rs)
}

func (s *searchResultSorter) Len() int {
	return len(s.Results)
}

func (s *searchResultSorter) Swap(i, j int) {
	s.Results[i], s.Results[j] = s.Results[j], s.Results[i]
}

func (s *searchResultSorter) Less(i, j int) bool {
	return s.By(&s.Results[i], &s.Results[j])
}

// Factory for search result comparison function. Either it takes index name
// into consideration or not.
func getSearchResultsCmpFunc(withIndex bool) by {
	// Compare two items in the result table of search command. First compare
	// the index we found the result in. Second compare their rating. Then
	// compare their fully qualified name (registry/name).
	less := func(fst, snd *registrytypes.SearchResultExt) bool {
		if withIndex {
			if fst.IndexName != snd.IndexName {
				return fst.IndexName < snd.IndexName
			}
			if fst.StarCount != snd.StarCount {
				return fst.StarCount > snd.StarCount
			}
		}
		if fst.RegistryName != snd.RegistryName {
			return fst.RegistryName < snd.RegistryName
		}
		if !withIndex {
			if fst.StarCount != snd.StarCount {
				return fst.StarCount > snd.StarCount
			}
		}
		if fst.Name != snd.Name {
			return fst.Name < snd.Name
		}
		return fst.Description < snd.Description
	}
	return less
}

func (s *Service) searchTerm(term string, authConfigs map[string]types.AuthConfig, headers map[string][]string, noIndex bool, outs *[]registrytypes.SearchResultExt) error {
	if err := validateNoSchema(term); err != nil {
		return err
	}

	indexName, remoteName := splitReposSearchTerm(term, true)

	index, err := newIndexInfo(s.Config, indexName)
	if err != nil {
		return err
	}

	// *TODO: Search multiple indexes.
	endpoint, err := NewEndpoint(index, http.Header(headers), APIVersionUnknown)
	if err != nil {
		return err
	}

	authConfig := ResolveAuthConfig(authConfigs, index)
	r, err := NewSession(endpoint.client, &authConfig, endpoint)
	if err != nil {
		return err
	}

	var results *registrytypes.SearchResults
	if index.Official {
		localName := remoteName
		if strings.HasPrefix(localName, reference.DefaultRepoPrefix) {
			// If pull "library/foo", it's stored locally under "foo"
			localName = strings.SplitN(localName, "/", 2)[1]
		}

		results, err = r.SearchRepositories(localName)
	} else {
		results, err = r.SearchRepositories(remoteName)
	}
	if err != nil || results.NumResults < 1 {
		return err
	}

	newOuts := make([]registrytypes.SearchResultExt, len(*outs)+len(results.Results))
	for i := range *outs {
		newOuts[i] = (*outs)[i]
	}
	for i, result := range results.Results {
		item := registrytypes.SearchResultExt{
			IndexName:    index.Name,
			RegistryName: index.Name,
			StarCount:    result.StarCount,
			Name:         result.Name,
			IsOfficial:   result.IsOfficial,
			IsTrusted:    result.IsTrusted,
			IsAutomated:  result.IsAutomated,
			Description:  result.Description,
		}
		// Check if search result is fully qualified with registry
		// If not, assume REGISTRY = INDEX
		newRegistryName, newName := splitReposSearchTerm(result.Name, false)
		if newRegistryName != "" {
			item.RegistryName, item.Name = newRegistryName, newName
		}
		newOuts[len(*outs)+i] = item
	}
	*outs = newOuts
	return nil
}

// Duplicate entries may occur in result table when omitting index from output because
// different indexes may refer to same registries.
func removeSearchDuplicates(data []registrytypes.SearchResultExt) []registrytypes.SearchResultExt {
	var (
		prevIndex = 0
		res       []registrytypes.SearchResultExt
	)

	if len(data) > 0 {
		res = []registrytypes.SearchResultExt{data[0]}
	}
	for i := 1; i < len(data); i++ {
		prev := res[prevIndex]
		curr := data[i]
		if prev.RegistryName == curr.RegistryName && prev.Name == curr.Name {
			// Repositories are equal, delete one of them.
			// Find out whose index has higher priority (the lower the number
			// the higher the priority).
			var prioPrev, prioCurr int
			for prioPrev = 0; prioPrev < len(DefaultRegistries); prioPrev++ {
				if prev.IndexName == DefaultRegistries[prioPrev] {
					break
				}
			}
			for prioCurr = 0; prioCurr < len(DefaultRegistries); prioCurr++ {
				if curr.IndexName == DefaultRegistries[prioCurr] {
					break
				}
			}
			if prioPrev > prioCurr || (prioPrev == prioCurr && prev.StarCount < curr.StarCount) {
				// replace previous entry with current one
				res[prevIndex] = curr
			} // otherwise keep previous entry
		} else {
			prevIndex++
			res = append(res, curr)
		}
	}
	return res
}

// Search queries several registries for images matching the specified
// search terms, and returns the results.
func (s *Service) Search(term string, authConfigs map[string]types.AuthConfig, headers map[string][]string, noIndex bool) ([]registrytypes.SearchResultExt, error) {
	results := []registrytypes.SearchResultExt{}
	cmpFunc := getSearchResultsCmpFunc(!noIndex)

	// helper for concurrent queries
	searchRoutine := func(term string, c chan<- error) {
		err := s.searchTerm(term, authConfigs, headers, noIndex, &results)
		c <- err
	}

	if isReposSearchTermFullyQualified(term) {
		if err := s.searchTerm(term, authConfigs, headers, noIndex, &results); err != nil {
			return nil, err
		}
	} else if len(DefaultRegistries) < 1 {
		return nil, fmt.Errorf("No configured repository to search.")
	} else {
		var (
			err              error
			successfulSearch = false
			resultChan       = make(chan error)
		)
		// query all registries in parallel
		for i, r := range DefaultRegistries {
			tmp := term
			if i > 0 {
				tmp = fmt.Sprintf("%s/%s", r, term)
			}
			go searchRoutine(tmp, resultChan)
		}
		for range DefaultRegistries {
			err = <-resultChan
			if err == nil {
				successfulSearch = true
			} else {
				logrus.Errorf("%s", err.Error())
			}
		}
		if !successfulSearch {
			return nil, err
		}
	}
	by(cmpFunc).Sort(results)
	if noIndex {
		results = removeSearchDuplicates(results)
	}
	return results, nil
}

// splitReposSearchTerm breaks a search term into an index name and remote name
func splitReposSearchTerm(reposName string, fixMissingIndex bool) (string, string) {
	nameParts := strings.SplitN(reposName, "/", 2)
	var indexName, remoteName string
	if len(nameParts) == 1 || (!strings.Contains(nameParts[0], ".") &&
		!strings.Contains(nameParts[0], ":") && nameParts[0] != "localhost") {
		// This is a Docker Index repos (ex: samalba/hipache or ubuntu)
		// 'docker.io'
		if fixMissingIndex {
			indexName = IndexServerName()
		} else {
			indexName = ""
		}
		remoteName = reposName
	} else {
		indexName = nameParts[0]
		remoteName = nameParts[1]
	}
	return indexName, remoteName
}

func isReposSearchTermFullyQualified(term string) bool {
	indexName, _ := splitReposSearchTerm(term, false)
	return indexName != ""
}

// ResolveRepository splits a repository name into its components
// and configuration of the associated registry.
func (s *Service) ResolveRepository(name reference.Named) (*RepositoryInfo, error) {
	return newRepositoryInfo(s.Config, name)
}

// ResolveIndex takes indexName and returns index info
func (s *Service) ResolveIndex(name string) (*registrytypes.IndexInfo, error) {
	return newIndexInfo(s.Config, name)
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
}

// ToV1Endpoint returns a V1 API endpoint based on the APIEndpoint
func (e APIEndpoint) ToV1Endpoint(metaHeaders http.Header) (*Endpoint, error) {
	return newEndpoint(e.URL, e.TLSConfig, metaHeaders)
}

// TLSConfig constructs a client TLS configuration based on server defaults
func (s *Service) TLSConfig(hostname string) (*tls.Config, error) {
	return newTLSConfig(hostname, isSecureIndex(s.Config, hostname))
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
func (s *Service) LookupPullEndpoints(repoName reference.Named) (endpoints []APIEndpoint, err error) {
	return s.lookupEndpoints(repoName)
}

// LookupPushEndpoints creates an list of endpoints to try to push to, in order of preference.
// It gives preference to v2 endpoints over v1, and HTTPS over plain HTTP.
// Mirrors are not included.
func (s *Service) LookupPushEndpoints(repoName reference.Named) (endpoints []APIEndpoint, err error) {
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

func (s *Service) lookupEndpoints(repoName reference.Named) (endpoints []APIEndpoint, err error) {
	endpoints, err = s.lookupV2Endpoints(repoName)
	if err != nil {
		return nil, err
	}

	if !V2Only {
		legacyEndpoints, err := s.lookupV1Endpoints(repoName)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, legacyEndpoints...)
	}

	filtered := filterBlockedEndpoints(endpoints)
	if len(filtered) == 0 && len(endpoints) > 0 {
		return nil, fmt.Errorf("All endpoints blocked.")
	}

	return filtered, nil
}
