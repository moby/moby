package distribution // import "github.com/docker/docker/distribution"

import (
	"context"
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/pkg/progress"
	refstore "github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Puller is an interface that abstracts pulling for different API versions.
type Puller interface {
	// Pull tries to pull the image referenced by `tag`
	// Pull returns an error if any, as well as a boolean that determines whether to retry Pull on the next configured endpoint.
	//
	Pull(ctx context.Context, ref reference.Named, platform *specs.Platform) error
}

// newPuller returns a Puller interface that will pull from a v2 registry.
func newPuller(endpoint registry.APIEndpoint, repoInfo *registry.RepositoryInfo, imagePullConfig *ImagePullConfig, local ContentStore) (Puller, error) {
	switch endpoint.Version {
	case registry.APIVersion2:
		return &v2Puller{
			V2MetadataService: metadata.NewV2MetadataService(imagePullConfig.MetadataStore),
			endpoint:          endpoint,
			config:            imagePullConfig,
			repoInfo:          repoInfo,
			manifestStore: &manifestStore{
				local: local,
			},
		}, nil
	case registry.APIVersion1:
		return nil, fmt.Errorf("protocol version %d no longer supported. Please contact admins of registry %s", endpoint.Version, endpoint.URL)
	}
	return nil, fmt.Errorf("unknown version %d for registry %s", endpoint.Version, endpoint.URL)
}

// Pull initiates a pull operation. image is the repository name to pull, and
// tag may be either empty, or indicate a specific tag to pull.
func Pull(ctx context.Context, ref reference.Named, imagePullConfig *ImagePullConfig, local ContentStore) error {
	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := imagePullConfig.RegistryService.ResolveRepository(ref)
	if err != nil {
		return err
	}

	// makes sure name is not `scratch`
	if err := ValidateRepoName(repoInfo.Name); err != nil {
		return err
	}

	endpoints, err := imagePullConfig.RegistryService.LookupPullEndpoints(reference.Domain(repoInfo.Name))
	if err != nil {
		return err
	}

	var (
		lastErr error

		// discardNoSupportErrors is used to track whether an endpoint encountered an error of type registry.ErrNoSupport
		// By default it is false, which means that if an ErrNoSupport error is encountered, it will be saved in lastErr.
		// As soon as another kind of error is encountered, discardNoSupportErrors is set to true, avoiding the saving of
		// any subsequent ErrNoSupport errors in lastErr.
		// It's needed for pull-by-digest on v1 endpoints: if there are only v1 endpoints configured, the error should be
		// returned and displayed, but if there was a v2 endpoint which supports pull-by-digest, then the last relevant
		// error is the ones from v2 endpoints not v1.
		discardNoSupportErrors bool

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

		logrus.Debugf("Trying to pull %s from %s %s", reference.FamiliarName(repoInfo.Name), endpoint.URL, endpoint.Version)

		puller, err := newPuller(endpoint, repoInfo, imagePullConfig, local)
		if err != nil {
			lastErr = err
			continue
		}

		if err := puller.Pull(ctx, ref, imagePullConfig.Platform); err != nil {
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
				if _, ok := err.(ErrNoSupport); !ok {
					// Because we found an error that's not ErrNoSupport, discard all subsequent ErrNoSupport errors.
					discardNoSupportErrors = true
					// append subsequent errors
					lastErr = err
				} else if !discardNoSupportErrors {
					// Save the ErrNoSupport error, because it's either the first error or all encountered errors
					// were also ErrNoSupport errors.
					// append subsequent errors
					lastErr = err
				}
				logrus.Infof("Attempting next endpoint for pull after error: %v", err)
				continue
			}
			logrus.Errorf("Not continuing with pull after error: %v", err)
			return TranslatePullError(err, ref)
		}

		imagePullConfig.ImageEventLogger(reference.FamiliarString(ref), reference.FamiliarName(repoInfo.Name), "pull")
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoints found for %s", reference.FamiliarString(ref))
	}

	return TranslatePullError(lastErr, ref)
}

// writeStatus writes a status message to out. If layersDownloaded is true, the
// status message indicates that a newer image was downloaded. Otherwise, it
// indicates that the image is up to date. requestedTag is the tag the message
// will refer to.
func writeStatus(requestedTag string, out progress.Output, layersDownloaded bool) {
	if layersDownloaded {
		progress.Message(out, "", "Status: Downloaded newer image for "+requestedTag)
	} else {
		progress.Message(out, "", "Status: Image is up to date for "+requestedTag)
	}
}

// ValidateRepoName validates the name of a repository.
func ValidateRepoName(name reference.Named) error {
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
