package registry

import (
	"github.com/dotcloud/docker/engine"
)

// Service exposes registry capabilities in the standard Engine
// interface. Once installed, it extends the engine with the
// following calls:
//
//  'auth': Authenticate against the public registry
//  'search': Search for images on the public registry (TODO)
//  'pull': Download images from any registry (TODO)
//  'push': Upload images to any registry (TODO)
type Service struct {
}

// NewService returns a new instance of Service ready to be
// installed no an engine.
func NewService() *Service {
	return &Service{}
}

// Install installs registry capabilities to eng.
func (s *Service) Install(eng *engine.Engine) error {
	eng.Register("auth", s.Auth)
	return nil
}

// Auth contacts the public registry with the provided credentials,
// and returns OK if authentication was sucessful.
// It can be used to verify the validity of a client's credentials.
func (s *Service) Auth(job *engine.Job) engine.Status {
	var (
		err        error
		authConfig = &AuthConfig{}
	)

	job.GetenvJson("authConfig", authConfig)
	// TODO: this is only done here because auth and registry need to be merged into one pkg
	if addr := authConfig.ServerAddress; addr != "" && addr != IndexServerAddress() {
		addr, err = ExpandAndVerifyRegistryUrl(addr)
		if err != nil {
			return job.Error(err)
		}
		authConfig.ServerAddress = addr
	}
	status, err := Login(authConfig, HTTPRequestFactory(nil))
	if err != nil {
		return job.Error(err)
	}
	job.Printf("%s\n", status)
	return engine.StatusOK
}
