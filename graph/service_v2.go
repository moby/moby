package graph

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/graph/tags"
	"github.com/docker/docker/image"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
	"golang.org/x/net/context"
)

type v2ManifestFetcher struct {
	*TagStore
	endpoint registry.APIEndpoint
	config   *LookupRemoteConfig
	repoInfo *registry.RepositoryInfo
	repo     distribution.Repository
}

func (p *v2ManifestFetcher) Fetch(ref string) (imgInspect *types.RemoteImageInspect, fallback bool, err error) {
	p.repo, err = NewV2Repository(p.repoInfo, p.endpoint, p.config.MetaHeaders, p.config.AuthConfig)
	if err != nil {
		logrus.Debugf("Error getting v2 registry: %v", err)
		return nil, true, err
	}

	imgInspect, err = p.fetchWithRepository(ref)
	if err != nil && registry.ContinueOnError(err) {
		logrus.Debugf("Error trying v2 registry: %v", err)
		fallback = true
	}
	return
}

func (p *v2ManifestFetcher) fetchWithRepository(ref string) (*types.RemoteImageInspect, error) {
	var (
		exists         bool
		dgst           digest.Digest
		err            error
		img            *image.Image
		signedManifest *manifest.SignedManifest
		tag            string
	)

	manSvc, err := p.repo.Manifests(context.Background())
	if err != nil {
		return nil, err
	}
	if utils.DigestReference(ref) {
		dgst, err = digest.ParseDigest(ref)
		if err != nil {
			return nil, fmt.Errorf("Invalid digest string %q: %v ", ref, err)
		}
		exists, err = manSvc.Exists(dgst)
		if err == nil && !exists {
			return nil, fmt.Errorf("Digest %q does not exist in remote repository %s", p.repoInfo.CanonicalName, p.repoInfo.CanonicalName)
		}
		if exists {
			signedManifest, err = manSvc.Get(dgst)
		}
	} else {
		if ref == "" {
			tagList, err := manSvc.Tags()
			if err != nil {
				return nil, err
			}
			for _, t := range tagList {
				if t == tags.DefaultTag {
					ref = tags.DefaultTag
				}
			}
			if ref == "" && len(tagList) > 0 {
				ref = tagList[0]
			}
			if ref == "" {
				return nil, fmt.Errorf("No tags available for remote repository %s", p.repoInfo.CanonicalName)
			}
		} else {
			exists, err = manSvc.ExistsByTag(ref)
			if err != nil {
				return nil, err
			}
			if err == nil && !exists {
				return nil, fmt.Errorf("Tag %q does not exist in remote repository %s", ref, p.repoInfo.CanonicalName)
			}
		}
		tag = ref
		signedManifest, err = manSvc.GetByTag(ref)
		if err == nil {
			dgst, _, err = digestFromManifest(signedManifest, p.repoInfo.LocalName)
		}
	}
	if err != nil {
		return nil, err
	}

	if len(signedManifest.Manifest.FSLayers) < 1 || len(signedManifest.Manifest.History) < 1 {
		return nil, fmt.Errorf("No layer in obtained manifest!")
	}
	imgJSON := []byte(signedManifest.Manifest.History[0].V1Compatibility)
	img, err = image.NewImgJSON(imgJSON)

	return makeRemoteImageInspect(p.repoInfo, img, tag, dgst), nil
}
