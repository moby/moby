package distribution

import (
	"context"

	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/errdefs"
)

// GetRepository returns a repository from the registry.
func GetRepository(ctx context.Context, ref reference.Named, config *ImagePullConfig) (repository distribution.Repository, lastError error) {
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

	for _, endpoint := range endpoints {
		repository, lastError = newRepository(ctx, repoInfo, endpoint, nil, config.AuthConfig, "pull")
		if lastError == nil {
			break
		}
	}
	return repository, lastError
}
