package distribution

import (
	"context"
	"fmt"

	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/events"
	refstore "github.com/moby/moby/v2/daemon/internal/refstore"
	"github.com/moby/moby/v2/daemon/pkg/registry"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// Pull initiates a pull operation. image is the repository name to pull, and
// tag may be either empty, or indicate a specific tag to pull.
func Pull(ctx context.Context, ref reference.Named, config *ImagePullConfig, local ContentStore) error {
	repoName, err := pullEndpoints(ctx, config.RegistryService, ref, func(ctx context.Context, repoName reference.Named, endpoint registry.APIEndpoint) error {
		log.G(ctx).Debugf("Trying to pull %s from %s", reference.FamiliarName(repoName), endpoint.URL)
		return newPuller(endpoint, repoName, config, local).pull(ctx, ref)
	})

	if err == nil {
		config.ImageEventLogger(ctx, reference.FamiliarString(ref), reference.FamiliarName(repoName), events.ActionPull)
	}

	return err
}

// Tags returns available tags for the given image in the remote repository.
func Tags(ctx context.Context, ref reference.Named, config *Config) ([]string, error) {
	var tags []string
	_, err := pullEndpoints(ctx, config.RegistryService, ref, func(ctx context.Context, repoName reference.Named, endpoint registry.APIEndpoint) error {
		repo, err := newRepository(ctx, repoName, endpoint, config.MetaHeaders, config.AuthConfig, "pull")
		if err != nil {
			return err
		}

		tags, err = repo.Tags(ctx).All(ctx)
		return err
	})

	return tags, err
}

// noBaseImageSpecifier is the symbol used by the FROM
// command to specify that no base image is to be used.
const noBaseImageSpecifier = "scratch"

// validateRepoName validates the name of a repository.
func validateRepoName(name reference.Named) error {
	if reference.FamiliarName(name) == noBaseImageSpecifier {
		return errors.WithStack(reservedNameError(noBaseImageSpecifier))
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
	} else if !errors.Is(err, refstore.ErrDoesNotExist) {
		return err
	}

	return store.AddDigest(dgstRef, id, true)
}

func pullEndpoints(ctx context.Context, registryService RegistryResolver, ref reference.Named,
	f func(context.Context, reference.Named, registry.APIEndpoint) error,
) (reference.Named, error) {
	repoName := reference.TrimNamed(ref)

	// makes sure name is not `scratch`
	if err := validateRepoName(repoName); err != nil {
		return repoName, err
	}

	endpoints, err := registryService.LookupPullEndpoints(reference.Domain(repoName))
	if err != nil {
		return repoName, err
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

		log.G(ctx).Debugf("Trying to pull %s from %s", reference.FamiliarName(repoName), endpoint.URL)

		if err := f(ctx, repoName, endpoint); err != nil {
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
			// FIXME(thaJeztah): cleanup error and context handling in this package, as it's really messy.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				log.G(ctx).WithError(err).Info("Not continuing with pull after error")
			} else {
				log.G(ctx).WithError(err).Error("Not continuing with pull after error")
			}
			return repoName, translatePullError(err, ref)
		}

		return repoName, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoints found for %s", reference.FamiliarString(ref))
	}

	return repoName, translatePullError(lastErr, ref)
}
