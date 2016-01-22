package distribution

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

// ImagePullConfig stores pull configuration.
type ImagePullConfig struct {
	// MetaHeaders stores HTTP headers with metadata about the image
	// (DockerHeaders with prefix X-Meta- in the request).
	MetaHeaders map[string][]string
	// AuthConfigs holds authentication credentials for authenticating with
	// the registries.
	AuthConfigs map[string]types.AuthConfig
	// ProgressOutput is the interface for showing the status of the pull
	// operation.
	ProgressOutput progress.Output
	// RegistryService is the registry service to use for TLS configuration
	// and endpoint lookup.
	RegistryService *registry.Service
	// ImageEventLogger notifies events for a given image
	ImageEventLogger func(id, name, action string)
	// MetadataStore is the storage backend for distribution-specific
	// metadata.
	MetadataStore metadata.Store
	// ImageStore manages images.
	ImageStore image.Store
	// ReferenceStore manages tags.
	ReferenceStore reference.Store
	// DownloadManager manages concurrent pulls.
	DownloadManager *xfer.LayerDownloadManager
}

// Puller is an interface that abstracts pulling for different API versions.
type Puller interface {
	// Pull tries to pull the image referenced by `tag`
	// Pull returns an error if any, as well as a boolean that determines whether to retry Pull on the next configured endpoint.
	//
	Pull(ctx context.Context, ref reference.Named) error
}

// newPuller returns a Puller interface that will pull from either a v1 or v2
// registry. The endpoint argument contains a Version field that determines
// whether a v1 or v2 puller will be created. The other parameters are passed
// through to the underlying puller implementation for use during the actual
// pull operation.
func newPuller(endpoint registry.APIEndpoint, repoInfo *registry.RepositoryInfo, imagePullConfig *ImagePullConfig) (Puller, error) {
	switch endpoint.Version {
	case registry.APIVersion2:
		return &v2Puller{
			V2MetadataService: metadata.NewV2MetadataService(imagePullConfig.MetadataStore),
			endpoint:          endpoint,
			config:            imagePullConfig,
			repoInfo:          repoInfo,
		}, nil
	case registry.APIVersion1:
		return &v1Puller{
			v1IDService: metadata.NewV1IDService(imagePullConfig.MetadataStore),
			endpoint:    endpoint,
			config:      imagePullConfig,
			repoInfo:    repoInfo,
		}, nil
	}
	return nil, fmt.Errorf("unknown version %d for registry %s", endpoint.Version, endpoint.URL)
}

// Pull initiates a pull operation for given reference. If the reference is
// fully qualified, image will be pulled from given registry. Otherwise
// additional registries will be queried until the reference is found.
func Pull(ctx context.Context, ref reference.Named, imagePullConfig *ImagePullConfig) error {
	// Unless the index name is specified, iterate over all registries until
	// the matching image is found.
	if reference.IsReferenceFullyQualified(ref) {
		return pullFromRegistry(ctx, ref, imagePullConfig)
	}
	if len(registry.DefaultRegistries) == 0 {
		return fmt.Errorf("No configured registry to pull from.")
	}
	err := validateRepoName(ref.Name())
	if err != nil {
		return err
	}
	for i, r := range registry.DefaultRegistries {
		// Prepend the index name to the image name.
		fqr, err := reference.QualifyUnqualifiedReference(ref, r)
		if err != nil {
			errStr := fmt.Sprintf("Failed to fully qualify %q name with %q registry: %v", ref.Name(), r, err)
			progress.Message(imagePullConfig.ProgressOutput, "", errStr)
			if i == len(registry.DefaultRegistries)-1 {
				return fmt.Errorf(errStr)
			}
			continue
		}
		if err := pullFromRegistry(ctx, fqr, imagePullConfig); err != nil {
			// make sure we get a final "Error response from daemon: "
			progress.Message(imagePullConfig.ProgressOutput, "", err.Error())
			if i == len(registry.DefaultRegistries)-1 {
				return err
			}
		} else {
			return nil
		}
	}

	return nil
}

// pullFromRegistry initiates a pull operation from particular registry. ref is
// a fully qualified image reference.
func pullFromRegistry(ctx context.Context, ref reference.Named, imagePullConfig *ImagePullConfig) error {
	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := imagePullConfig.RegistryService.ResolveRepository(ref)
	if err != nil {
		return err
	}

	progress.Messagef(imagePullConfig.ProgressOutput, "", "Trying to pull repository %s ... ", repoInfo.FullName())

	// makes sure name is not empty or `scratch`
	if err := validateRepoName(repoInfo.Name()); err != nil {
		return err
	}

	endpoints, err := imagePullConfig.RegistryService.LookupPullEndpoints(repoInfo)
	if err != nil {
		return err
	}

	var (
		// use a slice to append the error strings and return a joined string to caller
		errors []string

		// discardNoSupportErrors is used to track whether an endpoint encountered an error of type registry.ErrNoSupport
		// By default it is false, which means that if a ErrNoSupport error is encountered, it will be saved in errors.
		// As soon as another kind of error is encountered, discardNoSupportErrors is set to true, avoiding the saving of
		// any subsequent ErrNoSupport errors in errors.
		// It's needed for pull-by-digest on v1 endpoints: if there are only v1 endpoints configured, the error should be
		// returned and displayed, but if there was a v2 endpoint which supports pull-by-digest, then the last relevant
		// error is the ones from v2 endpoints not v1.
		discardNoSupportErrors bool

		// confirmedV2 is set to true if a pull attempt managed to
		// confirm that it was talking to a v2 registry. This will
		// prevent fallback to the v1 protocol.
		confirmedV2 bool
	)
	for _, endpoint := range endpoints {
		if confirmedV2 && endpoint.Version == registry.APIVersion1 {
			logrus.Debugf("Skipping v1 endpoint %s because v2 registry was detected", endpoint.URL)
			continue
		}
		logrus.Debugf("Trying to pull %s from %s %s", repoInfo.Name(), endpoint.URL, endpoint.Version)

		puller, err := newPuller(endpoint, repoInfo, imagePullConfig)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		if err := puller.Pull(ctx, ref); err != nil {
			// Was this pull cancelled? If so, don't try to fall
			// back.
			fallback := false
			select {
			case <-ctx.Done():
			default:
				if fallbackErr, ok := err.(fallbackError); ok {
					fallback = true
					confirmedV2 = confirmedV2 || fallbackErr.confirmedV2
					err = fallbackErr.err
				}
			}
			if fallback {
				if _, ok := err.(registry.ErrNoSupport); !ok {
					// Because we found an error that's not ErrNoSupport, discard all subsequent ErrNoSupport errors.
					discardNoSupportErrors = true
					// append subsequent errors
					errors = append(errors, err.Error())
				} else if !discardNoSupportErrors {
					// Save the ErrNoSupport error, because it's either the first error or all encountered errors
					// were also ErrNoSupport errors.
					// append subsequent errors
					errors = append(errors, err.Error())
				}
				continue
			}
			errors = append(errors, err.Error())
			if len(errors) > 0 {
				logrus.Debugf("Not continuing with error: %v", fmt.Errorf(strings.Join(errors, "\n")))
				return fmt.Errorf(strings.Join(errors, "\n"))
			}
		}

		imagePullConfig.ImageEventLogger(ref.String(), repoInfo.Name(), "pull")
		return nil
	}

	if len(errors) == 0 {
		return fmt.Errorf("no endpoints found for %s", ref.String())
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}
	return nil
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

// validateRepoName validates the name of a repository.
func validateRepoName(name string) error {
	if name == "" {
		return fmt.Errorf("Repository name can't be empty")
	}
	if strings.TrimPrefix(name, registry.IndexName+"/") == api.NoBaseImageSpecifier {
		return fmt.Errorf("'%s' is a reserved name", api.NoBaseImageSpecifier)
	}
	return nil
}

// tmpFileClose creates a closer function for a temporary file that closes the file
// and also deletes it.
func tmpFileCloser(tmpFile *os.File) func() error {
	return func() error {
		tmpFile.Close()
		if err := os.RemoveAll(tmpFile.Name()); err != nil {
			logrus.Errorf("Failed to remove temp file: %s", tmpFile.Name())
		}

		return nil
	}
}
