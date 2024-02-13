package distribution // import "github.com/docker/docker/distribution"

import (
	"context"
	"fmt"

	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types/events"
	refstore "github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// Pull initiates a pull operation. image is the repository name to pull, and
// tag may be either empty, or indicate a specific tag to pull.
func Pull(ctx context.Context, ref reference.Named, config *ImagePullConfig, local ContentStore) error {
	repoInfo, err := pullEndpoints(ctx, config.RegistryService, ref, func(ctx context.Context, repoInfo registry.RepositoryInfo, endpoint registry.APIEndpoint) error {
		log.G(ctx).Debugf("Trying to pull %s from %s", reference.FamiliarName(repoInfo.Name), endpoint.URL)
		puller := newPuller(endpoint, &repoInfo, config, local)
		return puller.pull(ctx, ref)
	})

	if err == nil {
		config.ImageEventLogger(reference.FamiliarString(ref), reference.FamiliarName(repoInfo.Name), events.ActionPull)
	}

	return err
}

// Tags returns available tags for the given image in the remote repository.
func Tags(ctx context.Context, ref reference.Named, config *Config) ([]string, error) {
	var tags []string
	_, err := pullEndpoints(ctx, config.RegistryService, ref, func(ctx context.Context, repoInfo registry.RepositoryInfo, endpoint registry.APIEndpoint) error {
		repo, err := newRepository(ctx, &repoInfo, endpoint, config.MetaHeaders, config.AuthConfig, "pull")
		if err != nil {
			return err
		}

		tags, err = repo.Tags(ctx).All(ctx)
		return err
	})

	return tags, err
}

// validateRepoName validates the name of a repository.
func validateRepoName(name reference.Named) error {
	if reference.FamiliarName(name) == api.NoBaseImageSpecifier {
		return errors.WithStack(reservedNameError(api.NoBaseImageSpecifier))
	}
	return nil
}

func addDigestReference(store refstore.Store, ref reference.Named, dgst digest.Digest, id digest.Digest) error {
	dgstRef, err := reference.WithDigest(reference.TrimNamed(ref), dgst)
	if err != nil {
		return err
	}

	if oldTagID, err := store.Get(dgstRef); err == nil {
		if oldTagID != id {
			// Updating digests not supported by reference store
			log.G(context.TODO()).Errorf("Image ID for digest %s changed from %s to %s, cannot update", dgst.String(), oldTagID, id)
		}
		return nil
	} else if err != refstore.ErrDoesNotExist {
		return err
	}

	return store.AddDigest(dgstRef, id, true)
}

func pullEndpoints(ctx context.Context, registryService RegistryResolver, ref reference.Named,
	f func(context.Context, registry.RepositoryInfo, registry.APIEndpoint) error,
) (*registry.RepositoryInfo, error) {
	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registryService.ResolveRepository(ref)
	if err != nil {
		return nil, err
	}

	// makes sure name is not `scratch`
	if err := validateRepoName(repoInfo.Name); err != nil {
		return repoInfo, err
	}

	endpoints, err := registryService.LookupPullEndpoints(reference.Domain(repoInfo.Name))
	if err != nil {
		return repoInfo, err
	}

	var (
		lastErr error

		// confirmedTLSRegistries is a map indicating which registries
		// are known to be using TLS. There should never be a plaintext
		// retry for any of these.
		confirmedTLSRegistries = make(map[string]struct{})
	)
	for _, endpoint := range endpoints {
		if endpoint.URL.Scheme != "https" {
			if _, confirmedTLS := confirmedTLSRegistries[endpoint.URL.Host]; confirmedTLS {
				log.G(ctx).Debugf("Skipping non-TLS endpoint %s for host/port that appears to use TLS", endpoint.URL)
				continue
			}
		}

		log.G(ctx).Debugf("Trying to pull %s from %s", reference.FamiliarName(repoInfo.Name), endpoint.URL)

		if err := f(ctx, *repoInfo, endpoint); err != nil {
			if _, ok := err.(fallbackError); !ok && continueOnError(err, endpoint.Mirror) {
				err = fallbackError{
					err:         err,
					transportOK: true,
				}
			}

			// Was this pull cancelled? If so, don't try to fall
			// back.
			fallback := false
			select {
			case <-ctx.Done():
			default:
				if fallbackErr, ok := err.(fallbackError); ok {
					fallback = true
					if fallbackErr.transportOK && endpoint.URL.Scheme == "https" {
						confirmedTLSRegistries[endpoint.URL.Host] = struct{}{}
					}
					err = fallbackErr.err
				}
			}
			if fallback {
				lastErr = err
				log.G(ctx).Infof("Attempting next endpoint for pull after error: %v", err)
				continue
			}
			log.G(ctx).Errorf("Not continuing with pull after error: %v", err)
			return repoInfo, translatePullError(err, ref)
		}

		return repoInfo, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoints found for %s", reference.FamiliarString(ref))
	}

	return repoInfo, translatePullError(lastErr, ref)
}
