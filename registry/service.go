package registry

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/Sirupsen/logrus"
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

// SearchResultExt describes a search result returned from a registry. It
// contains IndexName and RegistryName in addition to registry.SearchResult
// container.
type SearchResultExt struct {
	IndexName    string `json:"index_name"`
	RegistryName string `json:"registry_name"`
	StarCount    int    `json:"star_count"`
	IsOfficial   bool   `json:"is_official"`
	Name         string `json:"name"`
	IsTrusted    bool   `json:"is_trusted"`
	IsAutomated  bool   `json:"is_automated"`
	Description  string `json:"description"`
}

type by func(fst, snd *SearchResultExt) bool

type searchResultSorter struct {
	Results []SearchResultExt
	By      func(fst, snd *SearchResultExt) bool
}

func (by by) Sort(results []SearchResultExt) {
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
	less := func(fst, snd *SearchResultExt) bool {
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

func (s *Service) searchTerm(term string, authConfig *cliconfig.AuthConfig, headers map[string][]string, noIndex bool, outs *[]SearchResultExt) error {
	repoInfo, err := s.ResolveRepositoryBySearch(term)
	if err != nil {
		return err
	}

	// *TODO: Search multiple indexes.
	endpoint, err := NewEndpoint(repoInfo.Index, http.Header(headers), APIVersionUnknown)
	if err != nil {
		return err
	}
	r, err := NewSession(endpoint.client, authConfig, endpoint)
	if err != nil {
		return err
	}
	results, err := r.SearchRepositories(repoInfo.GetSearchTerm())
	if err != nil || results.NumResults < 1 {
		return err
	}
	newOuts := make([]SearchResultExt, len(*outs)+len(results.Results))
	for i := range *outs {
		newOuts[i] = (*outs)[i]
	}
	for i, result := range results.Results {
		item := SearchResultExt{
			IndexName:    repoInfo.Index.Name,
			RegistryName: repoInfo.Index.Name,
			StarCount:    result.StarCount,
			Name:         result.Name,
			IsOfficial:   result.IsOfficial,
			IsTrusted:    result.IsTrusted,
			IsAutomated:  result.IsAutomated,
			Description:  result.Description,
		}
		// Check if search result is fully qualified with registry
		// If not, assume REGISTRY = INDEX
		if RepositoryNameHasIndex(result.Name) {
			item.RegistryName, item.Name = SplitReposName(result.Name, false)
		}
		newOuts[len(*outs)+i] = item
	}
	*outs = newOuts
	return nil
}

// Duplicate entries may occur in result table when omitting index from output because
// different indexes may refer to same registries.
func removeSearchDuplicates(data []SearchResultExt) []SearchResultExt {
	var (
		prevIndex = 0
		res       []SearchResultExt
	)

	if len(data) > 0 {
		res = []SearchResultExt{data[0]}
	}
	for i := 1; i < len(data); i++ {
		prev := res[prevIndex]
		curr := data[i]
		if prev.RegistryName == curr.RegistryName && prev.Name == curr.Name {
			// Repositories are equal, delete one of them.
			// Find out whose index has higher priority (the lower the number
			// the higher the priority).
			var prioPrev, prioCurr int
			for prioPrev = 0; prioPrev < len(RegistryList); prioPrev++ {
				if prev.IndexName == RegistryList[prioPrev] {
					break
				}
			}
			for prioCurr = 0; prioCurr < len(RegistryList); prioCurr++ {
				if curr.IndexName == RegistryList[prioCurr] {
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
func (s *Service) Search(term string, authConfig *cliconfig.AuthConfig, headers map[string][]string, noIndex bool) ([]SearchResultExt, error) {
	results := []SearchResultExt{}
	cmpFunc := getSearchResultsCmpFunc(!noIndex)

	// helper for concurrent queries
	searchRoutine := func(term string, c chan<- error) {
		err := s.searchTerm(term, authConfig, headers, noIndex, &results)
		c <- err
	}

	if RepositoryNameHasIndex(term) {
		if err := s.searchTerm(term, authConfig, headers, noIndex, &results); err != nil {
			return nil, err
		}
	} else if len(RegistryList) < 1 {
		return nil, fmt.Errorf("No configured repository to search.")
	} else {
		var (
			err              error
			successfulSearch = false
			resultChan       = make(chan error)
		)
		// query all registries in parallel
		for i, r := range RegistryList {
			tmp := term
			if i > 0 {
				tmp = fmt.Sprintf("%s/%s", r, term)
			}
			go searchRoutine(tmp, resultChan)
		}
		for range RegistryList {
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
