package graph

import (
	"fmt"
	"io"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/registry"
)

// ImagePushConfig stores push configuration.
type ImagePushConfig struct {
	// MetaHeaders store HTTP headers with metadata about the image
	// (DockerHeaders with prefix X-Meta- in the request).
	MetaHeaders map[string][]string
	// AuthConfig holds authentication credentials for authenticating with
	// the registry.
	AuthConfig *cliconfig.AuthConfig
	// Tag is the specific variant of the image to be pushed.
	// If no tag is provided, all tags will be pushed.
	Tag string
	// OutStream is the formatted output writer for showing the status of the push
	// operation.
	OutStream *streamformatter.StdoutFormattedWriter
}

// NewImagePushConfig initializes ImagePushConfig structs with headers, auth configuration, a tag, and a json stream formatter.
func NewImagePushConfig(headers map[string][]string, authConfig *cliconfig.AuthConfig, tag string, out io.Writer) *ImagePushConfig {
	return &ImagePushConfig{
		MetaHeaders: headers,
		AuthConfig:  authConfig,
		Tag:         tag,
		OutStream:   streamformatter.NewStdoutJSONFormattedWriter(out),
	}
}

// Pusher is an interface that abstracts pushing for different API versions.
type Pusher interface {
	// Push tries to push the image configured at the creation of Pusher.
	// Push returns an error if any, as well as a boolean that determines whether to retry Push on the next configured endpoint.
	//
	// TODO(tiborvass): have Push() take a reference to repository + tag, so that the pusher itself is repository-agnostic.
	Push() (fallback bool, err error)
}

// NewPusher creates a new Pusher interface that will push to either a v1 or v2
// registry. The endpoint argument contains a Version field that determines
// whether a v1 or v2 pusher will be created. The other parameters are passed
// through to the underlying pusher implementation for use during the actual
// push operation.
func (s *TagStore) NewPusher(endpoint registry.APIEndpoint, localRepo Repository, repoInfo *registry.RepositoryInfo, imagePushConfig *ImagePushConfig) (Pusher, error) {
	switch endpoint.Version {
	case registry.APIVersion2:
		return &v2Pusher{
			TagStore:     s,
			endpoint:     endpoint,
			localRepo:    localRepo,
			repoInfo:     repoInfo,
			config:       imagePushConfig,
			layersPushed: make(map[digest.Digest]bool),
		}, nil
	case registry.APIVersion1:
		w := imagePushConfig.OutStream.Writer
		imagePushConfig.OutStream.Writer = ioutils.NewWriteFlusher(w)

		return &v1Pusher{
			TagStore:  s,
			endpoint:  endpoint,
			localRepo: localRepo,
			repoInfo:  repoInfo,
			config:    imagePushConfig,
		}, nil
	}
	return nil, fmt.Errorf("unknown version %d for registry %s", endpoint.Version, endpoint.URL)
}

// Push initiates a push operation on the repository named localName.
func (s *TagStore) Push(localName string, imagePushConfig *ImagePushConfig) error {
	// FIXME: Allow to interrupt current push when new push of same image is done.
	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := s.registryService.ResolveRepository(localName)
	if err != nil {
		return err
	}

	endpoints, err := s.registryService.LookupPushEndpoints(repoInfo.CanonicalName)
	if err != nil {
		return err
	}

	reposLen := 1
	if imagePushConfig.Tag == "" {
		reposLen = len(s.Repositories[repoInfo.LocalName])
	}

	imagePushConfig.OutStream.WriteStatus("", "The push refers to a repository [%s] (len: %d)", repoInfo.CanonicalName, reposLen)

	// If it fails, try to get the repository
	localRepo, exists := s.Repositories[repoInfo.LocalName]
	if !exists {
		return fmt.Errorf("Repository does not exist: %s", repoInfo.LocalName)
	}

	var lastErr error
	for _, endpoint := range endpoints {
		logrus.Debugf("Trying to push %s to %s %s", repoInfo.CanonicalName, endpoint.URL, endpoint.Version)

		pusher, err := s.NewPusher(endpoint, localRepo, repoInfo, imagePushConfig)
		if err != nil {
			lastErr = err
			continue
		}
		if fallback, err := pusher.Push(); err != nil {
			if fallback {
				lastErr = err
				continue
			}
			logrus.Debugf("Not continuing with error: %v", err)
			return err

		}

		s.eventsService.Log("push", repoInfo.LocalName, "")
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoints found for %s", repoInfo.CanonicalName)
	}
	return lastErr
}
