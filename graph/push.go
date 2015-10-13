package graph

import (
	"fmt"
	"io"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/cliconfig"
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
	// Whether to allow for push to public registry without confirmation.
	Force bool
	// OutStream is the output writer for showing the status of the push
	// operation.
	OutStream io.Writer
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
func (s *TagStore) NewPusher(endpoint registry.APIEndpoint, localRepo Repository, repoInfo *registry.RepositoryInfo, imagePushConfig *ImagePushConfig, sf *streamformatter.StreamFormatter) (Pusher, error) {
	switch endpoint.Version {
	case registry.APIVersion2:
		return &v2Pusher{
			TagStore:     s,
			endpoint:     endpoint,
			localRepo:    localRepo,
			repoInfo:     repoInfo,
			config:       imagePushConfig,
			sf:           sf,
			layersPushed: make(map[digest.Digest]bool),
		}, nil
	case registry.APIVersion1:
		return &v1Pusher{
			TagStore:  s,
			endpoint:  endpoint,
			localRepo: localRepo,
			repoInfo:  repoInfo,
			config:    imagePushConfig,
			sf:        sf,
		}, nil
	}
	return nil, fmt.Errorf("unknown version %d for registry %s", endpoint.Version, endpoint.URL)
}

// Push initiates a push operation on the repository named localName.
func (s *TagStore) Push(localName string, imagePushConfig *ImagePushConfig) error {
	// FIXME: Allow to interrupt current push when new push of same image is done.

	var (
		localRepo Repository
		sf        = streamformatter.NewJSONStreamFormatter()
	)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := s.registryService.ResolveRepository(localName)
	if err != nil {
		return err
	}

	// If we're not using a custom registry, we know the restrictions
	// applied to repository names and can warn the user in advance.
	// Custom repositories can have different rules, and we must also
	// allow pushing by image ID.
	if repoInfo.Official {
		username := imagePushConfig.AuthConfig.Username
		if username == "" {
			username = "<user>"
		}
		name := localName
		parts := strings.Split(repoInfo.LocalName, "/")
		if len(parts) > 0 {
			name = parts[len(parts)-1]
		}
		return fmt.Errorf("You cannot push a \"root\" repository. Please rename your repository to <user>/<repo> (ex: %s/%s)", username, name)
	}

	if repoInfo.Index.Official && s.ConfirmDefPush && !imagePushConfig.Force {
		return fmt.Errorf("Error: Status 403 trying to push repository %s to official registry: needs to be forced", localName)
	} else if repoInfo.Index.Official && !s.ConfirmDefPush && imagePushConfig.Force {
		logrus.Infof("Push of %s to official registry has been forced", localName)
	}

	endpoints, err := s.registryService.LookupPushEndpoints(repoInfo.CanonicalName)
	if err != nil {
		return err
	}

	reposLen := 1
	if imagePushConfig.Tag == "" {
		reposLen = len(s.Repositories[repoInfo.LocalName])
	}

	imagePushConfig.OutStream.Write(sf.FormatStatus("", "The push refers to a repository [%s] (len: %d)", repoInfo.CanonicalName, reposLen))
	matching := s.getRepositoryList(localName)
Loop:
	for _, namedRepo := range matching {
		for _, localRepo = range namedRepo {
			break Loop
		}
	}
	if localRepo == nil {
		return fmt.Errorf("Repository does not exist: %s", localName)
	}

	var lastErr error
	for _, endpoint := range endpoints {
		logrus.Debugf("Trying to push %s to %s %s", repoInfo.CanonicalName, endpoint.URL, endpoint.Version)

		pusher, err := s.NewPusher(endpoint, localRepo, repoInfo, imagePushConfig, sf)
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
