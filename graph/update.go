package graph

import (
	"fmt"
	"io"
	"strings"
	
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/image"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/streamformatter"
)

type ImagesUpdateConfig struct {
	DryRun    bool
	MetaHeaders map[string][]string
	AuthConfigs *map[string]registry.AuthConfig
	Json        bool
	OutStream   io.Writer
	Parallel bool
}

func (s *TagStore) Update(imagesUpdateConfig *ImagesUpdateConfig) error {
	var (
		sf = streamformatter.NewStreamFormatter(imagesUpdateConfig.Json)
		tags_to_pull []string
	)

	dry_run := imagesUpdateConfig.DryRun
	
	for name, repository := range s.Repositories {
		repoInfo, err :=  s.registryService.ResolveRepository(name)
		if err != nil {
			logrus.Errorf("Error: couldn't load repoInfo for %s: %s", name, err)
			continue
		}
		endpoint, err := repoInfo.GetEndpoint()
		if err != nil {
			logrus.Errorf("Error: coudln't find endpoint for %s: %s", name, err)
			continue
		}
		
		authConfig := s.ResolveAuthConfig(*imagesUpdateConfig.AuthConfigs, repoInfo.Index)
		
		r, err := registry.NewSession(&authConfig, registry.HTTPRequestFactory(imagesUpdateConfig.MetaHeaders), endpoint, true)
		if err != nil {
			logrus.Debugf("Warning: coudln't start http session for %s: %s", name, err)
			continue
		}
		
		//V2
		fallbackToV1 := true
		if len(repoInfo.Index.Mirrors) == 0 && ((repoInfo.Official && repoInfo.Index.Official) || endpoint.Version == registry.APIVersion2) {
			if repoInfo.Official {
				s.trustService.UpdateBase()
			}
	
			logrus.Debugf("checking v2 registry with local name %q", repoInfo.LocalName)
			
			tags_to_pull, err = s.checkV2Repository(r, imagesUpdateConfig.OutStream, repository, repoInfo, sf)
			if err == nil {
				fallbackToV1 = false
			} else if err != registry.ErrDoesNotExist && err != ErrV2RegistryUnavailable {
				logrus.Errorf("Error from V2 registry: %s", err)
			} else {
				logrus.Debug("image does not exist on v2 registry, falling back to v1") 	
			}
		}
		
		//V1
		if fallbackToV1 {
			tags_to_pull, err = s.checkRepository(r, imagesUpdateConfig.OutStream, repository, repoInfo, sf)
			if err != nil {
				imagesUpdateConfig.OutStream.Write(sf.FormatStatus(repoInfo.CanonicalName, err.Error(), nil))
				continue
			}
		}

		if len(tags_to_pull) == 0 || dry_run {
			if dry_run {
				logrus.Debugf("Dry run, not pulling anything")
			} else {
				logrus.Debugf("All images up to date")
			}
		} else {
			for _, tag_to_pull := range tags_to_pull {
			
			
				imagePullConfig := &ImagePullConfig{
					Parallel:    imagesUpdateConfig.Parallel,
					MetaHeaders: imagesUpdateConfig.MetaHeaders,
					AuthConfig:  &authConfig,
					OutStream:   imagesUpdateConfig.OutStream,
					Json:        imagesUpdateConfig.Json,
				}
	
				if err = s.Pull(name, tag_to_pull, imagePullConfig); err != nil {
					logrus.Errorf("Error pulling repository %s tag %s %s", name, tag_to_pull, err)
				}
			}
		}
	}
	
	return nil
}


func (s *TagStore) checkRepository(r *registry.Session, out io.Writer, repository Repository, repoInfo *registry.RepositoryInfo, sf *streamformatter.StreamFormatter) ([]string, error) {
	repoData, err := r.GetRepositoryData(repoInfo.RemoteName)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP code: 404") {
			return nil, fmt.Errorf("Error: image %s not found", repoInfo.RemoteName)
		}
		// Unexpected HTTP error
		return nil, err
	}

	out.Write(sf.FormatStatus(repoInfo.CanonicalName, "Checking repository, registry V1"))

	tags_to_pull := make([]string, 0, len(repository))	
	
	for tag, imgID := range repository {
		if tag == "" {
			continue
		}

		out.Write(sf.FormatProgress(repoInfo.CanonicalName, fmt.Sprintf("Checking for new version of image (%s)", tag), nil))
		success := false
		var err error
		var is_outdated bool

		for _, ep := range repoInfo.Index.Mirrors {
			if is_outdated, err = s.checkImageID(r, out, imgID, tag, repoInfo.RemoteName, ep, repoData.Tokens, sf); err != nil {
				// Don't report errors when pulling from mirrors.
				logrus.Debugf("Error checking for new version of image (%s) in %s, mirror: %s, %s", tag, repoInfo.CanonicalName, ep, err)
				continue
			}
			if is_outdated {
				tags_to_pull = append(tags_to_pull, tag)
			} 
			success = true
			break
		}
		if !success {
			for _, ep := range repoData.Endpoints {
				if is_outdated, err = s.checkImageID(r, out, imgID, tag, repoInfo.RemoteName, ep, repoData.Tokens, sf); err != nil {
					// It's not ideal that only the last error is returned, it would be better to concatenate the errors.
					// As the error is also given to the output stream the user will see the error.
					out.Write(sf.FormatProgress(stringid.TruncateID(imgID), fmt.Sprintf("Error checking for new version of image(%s) in %s, endpoint: %s, %s", tag, repoInfo.CanonicalName, ep, err), nil))
					continue
				}
				if is_outdated {
					tags_to_pull = append(tags_to_pull, tag)
				} 
				success = true
				break
			}
		}
		out.Write(sf.FormatProgress(stringid.TruncateID(imgID), "Checking complete", nil))
	}
	
	return tags_to_pull, nil
}

func (s *TagStore) checkV2Repository(r *registry.Session, out io.Writer, repository Repository, repoInfo *registry.RepositoryInfo, sf *streamformatter.StreamFormatter) ([]string, error) {
	endpoint, err := r.V2RegistryEndpoint(repoInfo.Index)
	if err != nil {
		if repoInfo.Index.Official {
			logrus.Debugf("Unable to pull from V2 registry, falling back to v1: %s", err)
			return nil, ErrV2RegistryUnavailable
		}
		return nil, fmt.Errorf("error getting registry endpoint: %s", err)
	}
	
	out.Write(sf.FormatStatus(repoInfo.CanonicalName, "Checking repository, registry V2"))
	auth, err := r.GetV2Authorization(endpoint, repoInfo.RemoteName, true)
	if err != nil {
		return nil, fmt.Errorf("error getting authorization: %s", err)
	}
	
	var is_outdated bool
	tags_to_pull := make([]string, 0, len(repository))	
	
	for tag, imgID := range repository {
		if is_outdated, err = s.checkImageIDV2(r, out, imgID, tag, repoInfo, endpoint, auth, sf); err != nil {
			logrus.Debugf("Error checking for new version of image (%s) in %s, mirror: %s, %s", tag, repoInfo.CanonicalName, endpoint, err)
			continue
		}
		if is_outdated {
			tags_to_pull = append(tags_to_pull, tag)
		}
	}
	return tags_to_pull, nil
}

func (s *TagStore) checkImageID(r *registry.Session, out io.Writer, imgID, tag string, repository_name string, endpoint string, token []string, sf *streamformatter.StreamFormatter) (bool, error) {
	remoteImgID, err := r.GetImageIDForTag(tag, repository_name, endpoint, token)
	if err != nil {
		return false, err
	}

	image_outdated := remoteImgID != imgID
	
	if image_outdated {
		out.Write(sf.FormatProgress(tag, fmt.Sprintf("Image is outdated, local id: %s, remote id: %s", imgID, remoteImgID), nil))
	} else {
		out.Write(sf.FormatProgress(tag, fmt.Sprintf("Image is up to date, id: %s", imgID), nil))
	}
	
	return image_outdated, nil
}

func (s *TagStore) checkImageIDV2(r *registry.Session, out io.Writer, imgID, tag string, repoInfo *registry.RepositoryInfo, endpoint *registry.Endpoint, auth *registry.RequestAuthorization, sf *streamformatter.StreamFormatter) (bool, error) {

	logrus.Debugf("Checking tag version in V2 registry: %q", tag)
	
	manifestBytes, manifestDigest, err := r.GetV2ImageManifest(endpoint, repoInfo.RemoteName, tag, auth)
	if err != nil {
		return false, err
	}

	// loadManifest ensures that the manifest payload has the expected digest
	// if the tag is a digest reference.
	manifest, verified, err := s.loadManifest(manifestBytes, manifestDigest, tag)
	if err != nil {
		return false, fmt.Errorf("error verifying manifest: %s", err)
	}

	if err := checkValidManifest(manifest); err != nil {
		return false, err
	}

	if verified {
		logrus.Printf("Image manifest for %s:%s has been verified", repoInfo.CanonicalName, tag)
	}

	for i := len(manifest.FSLayers) - 1; i >= 0; i-- {
		var (
			imgJSON = []byte(manifest.History[i].V1Compatibility)
		)

		img, err := image.NewImgJSON(imgJSON)
		if err != nil {
			return false, fmt.Errorf("failed to parse json: %s", err)
		}

		// Check if it doesn't exist
		if !s.graph.Exists(img.ID) {
			out.Write(sf.FormatProgress(tag, fmt.Sprintf("Image is outdated, local id: %s, remote id: %s", imgID, img.ID), nil))
			return true, nil
		}
	}
	
	out.Write(sf.FormatProgress(tag, fmt.Sprintf("Image is up to date, id: %s", imgID), nil))
	return false, nil
}

// this method matches a auth configuration to a server address or a url
func (s *TagStore) ResolveAuthConfig(configs map[string]registry.AuthConfig, index *registry.IndexInfo) registry.AuthConfig {
	configKey := index.GetAuthConfigKey()
	// First try the happy case
	if c, found := configs[configKey]; found || index.Official {
		return c
	}

	convertToHostname := func(url string) string {
		stripped := url
		if strings.HasPrefix(url, "http://") {
			stripped = strings.Replace(url, "http://", "", 1)
		} else if strings.HasPrefix(url, "https://") {
			stripped = strings.Replace(url, "https://", "", 1)
		}

		nameParts := strings.SplitN(stripped, "/", 2)

		return nameParts[0]
	}

	// Maybe they have a legacy config file, we will iterate the keys converting
	// them to the new format and testing
	for registry, config := range configs {
		if configKey == convertToHostname(registry) {
			return config
		}
	}

	// When all else fails, return an empty auth config
	return registry.AuthConfig{}
}
