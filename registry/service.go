package registry

import (
	"fmt"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
)

// Service exposes registry capabilities in the standard Engine
// interface. Once installed, it extends the engine with the
// following calls:
//
//  'auth': Authenticate against the public registry
//  'search': Search for images on the public registry
//  'pull': Download images from any registry (TODO)
//  'push': Upload images to any registry (TODO)
type Service struct {
	Config *ServiceConfig
}

// NewService returns a new instance of Service ready to be
// installed no an engine.
func NewService(options *Options) *Service {
	return &Service{
		Config: NewServiceConfig(options),
	}
}

// Install installs registry capabilities to eng.
func (s *Service) Install(eng *engine.Engine) error {
	eng.Register("auth", s.Auth)
	eng.Register("search", s.Search)
	eng.Register("resolve_repository", s.ResolveRepository)
	eng.Register("resolve_index", s.ResolveIndex)
	eng.Register("registry_config", s.GetRegistryConfig)
	return nil
}

// Auth contacts the public registry with the provided credentials,
// and returns OK if authentication was sucessful.
// It can be used to verify the validity of a client's credentials.
func (s *Service) Auth(job *engine.Job) error {
	var (
		authConfig = new(AuthConfig)
		endpoint   *Endpoint
		index      *IndexInfo
		status     string
		err        error
	)

	job.GetenvJson("authConfig", authConfig)

	addr := authConfig.ServerAddress
	if addr == "" {
		// Use the official registry address if not specified.
		addr = IndexServerAddress()
	}
	if addr == "" {
		return fmt.Errorf("No configured registry to authenticate to.")
	}

	if index, err = ResolveIndexInfo(job, addr); err != nil {
		return err
	}

	if endpoint, err = NewEndpoint(index); err != nil {
		logrus.Errorf("unable to get new registry endpoint: %s", err)
		return err
	}

	authConfig.ServerAddress = endpoint.String()

	if status, err = Login(authConfig, endpoint, HTTPRequestFactory(nil)); err != nil {
		logrus.Errorf("unable to login against registry endpoint %s: %s", endpoint, err)
		return err
	}

	logrus.Infof("successful registry login for endpoint %s: %s", endpoint, status)
	job.Printf("%s\n", status)

	return nil
}

// Factory for search result comparison function. Either it takes index name
// into consideration or not.
func getSearchResultsCmpFunc(withIndex bool) func(fst, snd *engine.Env) int {
	cmpByStarCount := func(fst, snd *engine.Env) int {
		starsA := fst.Get("star_count")
		starsB := snd.Get("star_count")

		intA, errA := strconv.ParseInt(starsA, 10, 64)
		intB, errB := strconv.ParseInt(starsB, 10, 64)
		if errA == nil && errB == nil {
			switch {
			case intA > intB:
				return -1
			case intA < intB:
				return 1
			}
		}
		switch {
		case starsA > starsB:
			return -1
		case starsA < starsB:
			return 1
		}
		return 0
	}

	cmpStringField := func(field string, fst, snd *engine.Env) int {
		valA := fst.Get(field)
		valB := snd.Get(field)
		switch {
		case valA < valB:
			return -1
		case valA > valB:
			return 1
		}
		return 0
	}

	// Compare two items in the result table of search command. First compare
	// the index we found the result in. Second compare their rating. Then
	// compare their fully qualified name (registry/name).
	cmpFunc := func(fst, snd *engine.Env) int {
		if withIndex {
			if res := cmpStringField("index_name", fst, snd); res != 0 {
				return res
			}
			if byStarCount := cmpByStarCount(fst, snd); byStarCount != 0 {
				return byStarCount
			}
		}
		if res := cmpStringField("registry_name", fst, snd); res != 0 {
			return res
		}
		if !withIndex {
			if byStarCount := cmpByStarCount(fst, snd); byStarCount != 0 {
				return byStarCount
			}
		}
		if res := cmpStringField("name", fst, snd); res != 0 {
			return res
		}
		if res := cmpStringField("description", fst, snd); res != 0 {
			return res
		}
		return 0
	}
	return cmpFunc
}

func searchTerm(job *engine.Job, outs *engine.Table, term string) error {
	var (
		metaHeaders = map[string][]string{}
		authConfig  = &AuthConfig{}
	)
	job.GetenvJson("authConfig", authConfig)
	job.GetenvJson("metaHeaders", metaHeaders)
	noIndex := job.GetenvBool("noIndex")

	repoInfo, err := ResolveRepositoryInfo(job, term)
	if err != nil {
		return err
	}
	endpoint, err := repoInfo.GetEndpoint()
	if err != nil {
		return err
	}
	r, err := NewSession(authConfig, HTTPRequestFactory(metaHeaders), endpoint, true)
	if err != nil {
		return err
	}
	results, err := r.SearchRepositories(repoInfo.GetSearchTerm())
	if err != nil {
		return err
	}
	for _, result := range results.Results {
		out := &engine.Env{}
		// Check if search result has is fully qualified with registry
		// If not, assume REGISTRY = INDEX
		registryName := repoInfo.Index.Name
		if RepositoryNameHasIndex(result.Name) {
			registryName, result.Name = SplitReposName(result.Name, false)
		}
		out.Import(result)
		// Now add the index in which we found the result to the json. (not sure this is really the right place for this)
		out.Set("registry_name", registryName)
		if !noIndex {
			out.Set("index_name", repoInfo.Index.Name)
		}
		outs.Add(out)
	}
	return nil
}

// Duplicate entries may occur in result table when omitting index from output because
// different indexes may refer to same registries.
func removeSearchDuplicates(data []*engine.Env) (res []*engine.Env) {
	var prevIndex = 0

	if len(data) > 0 {
		res = []*engine.Env{data[0]}
	}
	for i := 1; i < len(data); i++ {
		prev := res[prevIndex]
		curr := data[i]
		if prev.Get("registry_name") == curr.Get("registry_name") && prev.Get("name") == curr.Get("name") {
			// Repositories are equal, delete one of them.
			// Find out whose index has higher priority (the lower the number
			// the higher the priority).
			var prioPrev, prioCurr int
			for prioPrev = 0; prioPrev < len(RegistryList); prioPrev++ {
				if prev.Get("index_name") == RegistryList[prioPrev] {
					break
				}
			}
			for prioCurr = 0; prioCurr < len(RegistryList); prioCurr++ {
				if curr.Get("index_name") == RegistryList[prioCurr] {
					break
				}
			}
			if prioPrev > prioCurr || (prioPrev == prioCurr && prev.Get("star_count") < curr.Get("star_count")) {
				// replace previous entry with current one
				res[prevIndex] = curr
			} // otherwise keep previous entry
		} else {
			prevIndex++
			res = append(res, curr)
		}
	}
	return
}

// Search queries the public registry for images matching the specified
// search terms, and returns the results.
//
// Argument syntax: search TERM
//
// Option environment:
//	'authConfig': json-encoded credentials to authenticate against the registry.
//		The search extends to images only accessible via the credentials.
//
//	'metaHeaders': extra HTTP headers to include in the request to the registry.
//		The headers should be passed as a json-encoded dictionary.
//	'noIndex': boolean parameter saying wether to include index in results or
//		not
//
// Output:
//	Results are sent as a collection of structured messages (using engine.Table).
//	Each result is sent as a separate message.
//	Results are ordered by:
//		1. registry's index name
//		2. number of stars on registry
//		3. registry's name
func (s *Service) Search(job *engine.Job) error {
	if n := len(job.Args); n != 1 {
		return fmt.Errorf("Usage: %s TERM", job.Name)
	}
	var (
		term    = job.Args[0]
		noIndex = job.GetenvBool("noIndex")
		outs    = engine.NewTableWithCmpFunc(getSearchResultsCmpFunc(!noIndex), 0)
	)

	// helper for concurrent queries
	searchRoutine := func(term string, c chan<- error) {
		err := searchTerm(job, outs, term)
		c <- err
	}

	if RepositoryNameHasIndex(term) {
		if err := searchTerm(job, outs, term); err != nil {
			return err
		}
	} else if len(RegistryList) < 1 {
		return fmt.Errorf("No configured repository to search.")
	} else {
		var (
			err              error
			successfulSearch = false
			resultChan       = make(chan error)
		)
		// query all registries in parallel
		for i, r := range RegistryList {
			if i > 0 {
				job.Args[0] = fmt.Sprintf("%s/%s", r, term)
			} else {
				job.Args[0] = term
			}
			go searchRoutine(job.Args[0], resultChan)
		}
		for _ = range RegistryList {
			err = <-resultChan
			if err == nil {
				successfulSearch = true
			} else {
				logrus.Errorf("%s", err.Error())
			}
		}
		if !successfulSearch {
			return err
		}
	}
	outs.Sort()
	if noIndex {
		outs.Data = removeSearchDuplicates(outs.Data)
	}
	if _, err := outs.WriteListTo(job.Stdout); err != nil {
		return err
	}
	return nil
}

// ResolveRepository splits a repository name into its components
// and configuration of the associated registry.
func (s *Service) ResolveRepository(job *engine.Job) error {
	var (
		reposName = job.Args[0]
	)

	repoInfo, err := s.Config.NewRepositoryInfo(reposName)
	if err != nil {
		return err
	}

	out := engine.Env{}
	err = out.SetJson("repository", repoInfo)
	if err != nil {
		return err
	}
	out.WriteTo(job.Stdout)

	return nil
}

// Convenience wrapper for calling resolve_repository Job from a running job.
func ResolveRepositoryInfo(jobContext *engine.Job, reposName string) (*RepositoryInfo, error) {
	job := jobContext.Eng.Job("resolve_repository", reposName)
	env, err := job.Stdout.AddEnv()
	if err != nil {
		return nil, err
	}
	if err := job.Run(); err != nil {
		return nil, err
	}
	info := RepositoryInfo{}
	if err := env.GetJson("repository", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// ResolveIndex takes indexName and returns index info
func (s *Service) ResolveIndex(job *engine.Job) error {
	var (
		indexName = job.Args[0]
	)

	index, err := s.Config.NewIndexInfo(indexName)
	if err != nil {
		return err
	}

	out := engine.Env{}
	err = out.SetJson("index", index)
	if err != nil {
		return err
	}
	out.WriteTo(job.Stdout)

	return nil
}

// Convenience wrapper for calling resolve_index Job from a running job.
func ResolveIndexInfo(jobContext *engine.Job, indexName string) (*IndexInfo, error) {
	job := jobContext.Eng.Job("resolve_index", indexName)
	env, err := job.Stdout.AddEnv()
	if err != nil {
		return nil, err
	}
	if err := job.Run(); err != nil {
		return nil, err
	}
	info := IndexInfo{}
	if err := env.GetJson("index", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// GetRegistryConfig returns current registry configuration.
func (s *Service) GetRegistryConfig(job *engine.Job) error {
	out := engine.Env{}
	err := out.SetJson("config", s.Config)
	if err != nil {
		return err
	}
	out.WriteTo(job.Stdout)

	return nil
}
