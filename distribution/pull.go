package distribution // import "github.com/docker/docker/distribution"

import (
	"context"
	"fmt"

	reference "github.com/containerd/containerd/reference/docker"
	"github.com/docker/docker/api"
	refstore "github.com/docker/docker/reference"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Pull initiates a pull operation. image is the repository name to pull, and
// tag may be either empty, or indicate a specific tag to pull.
func Pull(ctx context.Context, ref reference.Named, config *ImagePullConfig, local ContentStore) error {
	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := config.RegistryService.ResolveRepository(ref)
	if err != nil {
		return err
	}

	// makes sure name is not `scratch`
	if err := validateRepoName(repoInfo.Name); err != nil {
		return err
	}

	endpoints, err := config.RegistryService.LookupPullEndpoints(reference.Domain(repoInfo.Name))
	if err != nil {
		return err
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
				logrus.Debugf("Skipping non-TLS endpoint %s for host/port that appears to use TLS", endpoint.URL)
				continue
			}
		}

		logrus.Debugf("Trying to pull %s from %s", reference.FamiliarName(repoInfo.Name), endpoint.URL)

		if err := newPuller(endpoint, repoInfo, config, local).pull(ctx, ref); err != nil {
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
				logrus.Infof("Attempting next endpoint for pull after error: %v", err)
				continue
			}
			logrus.Errorf("Not continuing with pull after error: %v", err)
			return translatePullError(err, ref)
		}

		config.ImageEventLogger(reference.FamiliarString(ref), reference.FamiliarName(repoInfo.Name), "pull")
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoints found for %s", reference.FamiliarString(ref))
	}

	return translatePullError(lastErr, ref)
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
			logrus.Errorf("Image ID for digest %s changed from %s to %s, cannot update", dgst.String(), oldTagID, id)
		}
		return nil
	} else if err != refstore.ErrDoesNotExist {
		return err
	}

	return store.AddDigest(dgstRef, id, true)
}
