package distribution

import (
	"context"

	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/docker/docker/errdefs"
)

// GetRepositories returns a list of repositories configured for the given
// reference. Multiple repositories can be returned if the reference is for
// the default (Docker Hub) registry and a mirror is configured, but it omits
// registries that were not reachable (pinging the /v2/ endpoint failed).
//
// It returns an error if it was unable to reach any of the registries for
// the given reference, or if the provided reference is invalid.
func GetRepositories(ctx context.Context, ref reference.Named, config *ImagePullConfig) ([]distribution.Repository, error) {
	repoInfo, err := config.RegistryService.ResolveRepository(ref)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}
	// makes sure name is not empty or `scratch`
	if err := validateRepoName(repoInfo.Name); err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	endpoints, err := config.RegistryService.LookupPullEndpoints(reference.Domain(repoInfo.Name))
	if err != nil {
		return nil, err
	}

	var (
		repositories []distribution.Repository
		lastError    error
	)
	for _, endpoint := range endpoints {
		repo, err := newRepository(ctx, repoInfo, endpoint, nil, config.AuthConfig, "pull")
		if err != nil {
			log.G(ctx).WithFields(log.Fields{"endpoint": endpoint.URL.String(), "error": err}).Info("endpoint")
			lastError = err
			continue
		}
		repositories = append(repositories, repo)
	}
	if len(repositories) == 0 {
		return nil, lastError
	}
	return repositories, nil
}
