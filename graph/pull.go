package graph

import (
	"fmt"
	"io"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

// ImagePullConfig stores pull configuration.
type ImagePullConfig struct {
	// MetaHeaders stores HTTP headers with metadata about the image
	// (DockerHeaders with prefix X-Meta- in the request).
	MetaHeaders map[string][]string
	// AuthConfig holds authentication credentials for authenticating with
	// the registry.
	AuthConfig *cliconfig.AuthConfig
	// OutStream is the output writer for showing the status of the pull
	// operation.
	OutStream io.Writer
}

// Puller is an interface that abstracts pulling for different API versions.
type Puller interface {
	// Pull tries to pull the image referenced by `tag`
	// Pull returns an error if any, as well as a boolean that determines whether to retry Pull on the next configured endpoint.
	//
	// TODO(tiborvass): have Pull() take a reference to repository + tag, so that the puller itself is repository-agnostic.
	Pull(tag string) (fallback bool, err error)
}

// NewPuller returns a Puller interface that will pull from either a v1 or v2
// registry. The endpoint argument contains a Version field that determines
// whether a v1 or v2 puller will be created. The other parameters are passed
// through to the underlying puller implementation for use during the actual
// pull operation.
func NewPuller(s *TagStore, endpoint registry.APIEndpoint, repoInfo *registry.RepositoryInfo, imagePullConfig *ImagePullConfig, sf *streamformatter.StreamFormatter) (Puller, error) {
	switch endpoint.Version {
	case registry.APIVersion2:
		return &v2Puller{
			TagStore: s,
			endpoint: endpoint,
			config:   imagePullConfig,
			sf:       sf,
			repoInfo: repoInfo,
		}, nil
	case registry.APIVersion1:
		return &v1Puller{
			TagStore: s,
			endpoint: endpoint,
			config:   imagePullConfig,
			sf:       sf,
			repoInfo: repoInfo,
		}, nil
	}
	return nil, fmt.Errorf("unknown version %d for registry %s", endpoint.Version, endpoint.URL)
}

// Pull initiates a pull operation. image is the repository name to pull, and
// tag may be either empty, or indicate a specific tag to pull.
func (s *TagStore) Pull(image string, tag string, imagePullConfig *ImagePullConfig) error {
	var err error
	doPull := func(image string) error {
		err := s.pullFromRegistry(image, tag, imagePullConfig)
		return err
	}
	// Unless the index name is specified, iterate over all registries until
	// the matching image is found.
	if registry.RepositoryNameHasIndex(image) {
		return doPull(image)
	}
	if len(registry.RegistryList) == 0 {
		return fmt.Errorf("No configured registry to pull from.")
	}
	for _, r := range registry.RegistryList {
		// Prepend the index name to the image name.
		if err = doPull(fmt.Sprintf("%s/%s", r, image)); err == nil {
			return nil
		}
	}
	return err
}

func (s *TagStore) pullFromRegistry(image string, tag string, imagePullConfig *ImagePullConfig) error {
	var (
		sf  = streamformatter.NewJSONStreamFormatter()
		out = imagePullConfig.OutStream
	)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := s.registryService.ResolveRepository(image)
	if err != nil {
		return err
	}

	out.Write(sf.FormatStream(fmt.Sprintf("Trying to pull repository %s ... ", repoInfo.CanonicalName)))

	// makes sure name is not empty or `scratch`
	if err := validateRepoName(repoInfo.LocalName); err != nil {
		out.Write(sf.FormatStatus("", "failed"))
		return err
	}

	endpoints, err := s.registryService.LookupPullEndpoints(repoInfo.CanonicalName)
	if err != nil {
		out.Write(sf.FormatStatus("", "failed"))
		return err
	}

	logName := repoInfo.LocalName
	if tag != "" {
		logName = utils.ImageReference(logName, tag)
	}

	var (
		lastErr error

		// discardNoSupportErrors is used to track whether an endpoint encountered an error of type registry.ErrNoSupport
		// By default it is false, which means that if a ErrNoSupport error is encountered, it will be saved in lastErr.
		// As soon as another kind of error is encountered, discardNoSupportErrors is set to true, avoiding the saving of
		// any subsequent ErrNoSupport errors in lastErr.
		// It's needed for pull-by-digest on v1 endpoints: if there are only v1 endpoints configured, the error should be
		// returned and displayed, but if there was a v2 endpoint which supports pull-by-digest, then the last relevant
		// error is the ones from v2 endpoints not v1.
		discardNoSupportErrors bool
	)
	for _, endpoint := range endpoints {
		logrus.Debugf("Trying to pull %s from %s %s", repoInfo.LocalName, endpoint.URL, endpoint.Version)

		puller, err := NewPuller(s, endpoint, repoInfo, imagePullConfig, sf)
		if err != nil {
			lastErr = err
			continue
		}
		if fallback, err := puller.Pull(tag); err != nil {
			if fallback {
				if _, ok := err.(registry.ErrNoSupport); !ok {
					// Because we found an error that's not ErrNoSupport, discard all subsequent ErrNoSupport errors.
					discardNoSupportErrors = true
					// save the current error
					lastErr = err
				} else if !discardNoSupportErrors {
					// Save the ErrNoSupport error, because it's either the first error or all encountered errors
					// were also ErrNoSupport errors.
					lastErr = err
				}
				continue
			}
			logrus.Debugf("Not continuing with error: %v", err)
			if strings.Contains(err.Error(), "not found") {
				out.Write(sf.FormatStatus("", "not found"))
			} else {
				out.Write(sf.FormatStatus("", "failed"))
			}
			return err

		}

		s.eventsService.Log("pull", logName, "")
		out.Write(sf.FormatStatus("", ""))
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoints found for %s", image)
	}
	if strings.Contains(lastErr.Error(), "not found") {
		out.Write(sf.FormatStatus("", "not found"))
	} else {
		out.Write(sf.FormatStatus("", "failed"))
	}
	return lastErr
}

// writeStatus writes a status message to out. If layersDownloaded is true, the
// status message indicates that a newer image was downloaded. Otherwise, it
// indicates that the image is up to date. requestedTag is the tag the message
// will refer to.
func writeStatus(requestedTag string, out io.Writer, sf *streamformatter.StreamFormatter, layersDownloaded bool) {
	if layersDownloaded {
		out.Write(sf.FormatStatus("", "Status: Downloaded newer image for %s", requestedTag))
	} else {
		out.Write(sf.FormatStatus("", "Status: Image is up to date for %s", requestedTag))
	}
}
