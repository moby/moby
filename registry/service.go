package registry

import (
	log "github.com/Sirupsen/logrus"
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
func (s *Service) Auth(job *engine.Job) engine.Status {
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

	if index, err = ResolveIndexInfo(job, addr); err != nil {
		return job.Error(err)
	}

	if endpoint, err = NewEndpoint(index); err != nil {
		log.Errorf("unable to get new registry endpoint: %s", err)
		return job.Error(err)
	}

	authConfig.ServerAddress = endpoint.String()

	if status, err = Login(authConfig, endpoint, HTTPRequestFactory(nil)); err != nil {
		log.Errorf("unable to login against registry endpoint %s: %s", endpoint, err)
		return job.Error(err)
	}

	log.Infof("successful registry login for endpoint %s: %s", endpoint, status)
	job.Printf("%s\n", status)

	return engine.StatusOK
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
//
// Output:
//	Results are sent as a collection of structured messages (using engine.Table).
//	Each result is sent as a separate message.
//	Results are ordered by number of stars on the public registry.
func (s *Service) Search(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: %s TERM", job.Name)
	}
	var (
		term        = job.Args[0]
		metaHeaders = map[string][]string{}
		authConfig  = &AuthConfig{}
	)
	job.GetenvJson("authConfig", authConfig)
	job.GetenvJson("metaHeaders", metaHeaders)

	repoInfo, err := ResolveRepositoryInfo(job, term)
	if err != nil {
		return job.Error(err)
	}
	// *TODO: Search multiple indexes.
	endpoint, err := repoInfo.GetEndpoint()
	if err != nil {
		return job.Error(err)
	}
	r, err := NewSession(authConfig, HTTPRequestFactory(metaHeaders), endpoint, true)
	if err != nil {
		return job.Error(err)
	}
	results, err := r.SearchRepositories(repoInfo.GetSearchTerm())
	if err != nil {
		return job.Error(err)
	}
	outs := engine.NewTable("star_count", 0)
	for _, result := range results.Results {
		out := &engine.Env{}
		out.Import(result)
		outs.Add(out)
	}
	outs.ReverseSort()
	if _, err := outs.WriteListTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

// ResolveRepository splits a repository name into its components
// and configuration of the associated registry.
func (s *Service) ResolveRepository(job *engine.Job) engine.Status {
	var (
		reposName = job.Args[0]
	)

	repoInfo, err := s.Config.NewRepositoryInfo(reposName)
	if err != nil {
		return job.Error(err)
	}

	out := engine.Env{}
	err = out.SetJson("repository", repoInfo)
	if err != nil {
		return job.Error(err)
	}
	out.WriteTo(job.Stdout)

	return engine.StatusOK
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
func (s *Service) ResolveIndex(job *engine.Job) engine.Status {
	var (
		indexName = job.Args[0]
	)

	index, err := s.Config.NewIndexInfo(indexName)
	if err != nil {
		return job.Error(err)
	}

	out := engine.Env{}
	err = out.SetJson("index", index)
	if err != nil {
		return job.Error(err)
	}
	out.WriteTo(job.Stdout)

	return engine.StatusOK
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
func (s *Service) GetRegistryConfig(job *engine.Job) engine.Status {
	out := engine.Env{}
	err := out.SetJson("config", s.Config)
	if err != nil {
		return job.Error(err)
	}
	out.WriteTo(job.Stdout)

	return engine.StatusOK
}
