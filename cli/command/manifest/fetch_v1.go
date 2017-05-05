package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types"
	dockerdistribution "github.com/docker/docker/distribution"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/image/v1"
	"github.com/docker/docker/registry"
	"golang.org/x/net/context"
)

type v1ManifestFetcher struct {
	endpoint    registry.APIEndpoint
	repoInfo    *registry.RepositoryInfo
	repo        distribution.Repository
	confirmedV2 bool
	// wrap in a config?
	authConfig types.AuthConfig
	// Leaving this as a pointer to an interface won't compile for me
	service registry.Service
	session *registry.Session
}

func (mf *v1ManifestFetcher) Fetch(ctx context.Context, ref reference.Named) ([]ImgManifestInspect, error) {
	// @TODO: Re-test the v1 registry stuff after pulling in all the consolidated reference stuff.
	// Pre-condition: ref has to be tagged (e.g. using ParseNormalizedNamed)
	if _, isCanonical := ref.(reference.Canonical); isCanonical {
		// Allowing fallback, because HTTPS v1 is before HTTP v2
		return nil, fallbackError{
			err: dockerdistribution.ErrNoSupport{errors.New("Cannot pull by digest with v1 registry")},
		}
	}
	tlsConfig, err := mf.service.TLSConfig(mf.repoInfo.Index.Name)
	if err != nil {
		return nil, err
	}
	// Adds Docker-specific headers as well as user-specified headers (metaHeaders)
	tr := transport.NewTransport(
		registry.NewTransport(tlsConfig),
		//registry.DockerHeaders(mf.config.MetaHeaders)...,
		registry.DockerHeaders(dockerversion.DockerUserAgent(nil), nil)...,
	)
	client := registry.HTTPClient(tr)
	//v1Endpoint, err := mf.endpoint.ToV1Endpoint(mf.config.MetaHeaders)
	v1Endpoint, err := mf.endpoint.ToV1Endpoint(dockerversion.DockerUserAgent(nil), nil)
	if err != nil {
		logrus.Debugf("Could not get v1 endpoint: %v", err)
		return nil, fallbackError{err: err}
	}
	mf.session, err = registry.NewSession(client, &mf.authConfig, v1Endpoint)
	if err != nil {
		logrus.Debugf("Fallback from error: %s", err)
		return nil, fallbackError{err: err}
	}

	imgsInspect, err := mf.fetchWithSession(ctx, ref)
	if err != nil {
		return nil, err
	}
	if len(imgsInspect) > 1 {
		return nil, fmt.Errorf("Found more than one image in V1 fetch!? %v", imgsInspect)
	}
	imgsInspect[0].MediaType = schema1.MediaTypeManifest
	return imgsInspect, nil
}

func (mf *v1ManifestFetcher) fetchWithSession(ctx context.Context, ref reference.Named) ([]ImgManifestInspect, error) {
	// Pre-Condition: ref should always be tagged (e.g. using ParseNormalizedNamed)
	var (
		imageList = []ImgManifestInspect{}
		pulledImg *image.Image
		tagsMap   map[string]string
	)
	logrus.Debugf("Fetching v1 manifest for %s", mf.repoInfo.Name)
	repoData, err := mf.session.GetRepositoryData(mf.repoInfo.Name)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP code: 404") {
			return nil, fmt.Errorf("Error: image %s not found", reference.Path(mf.repoInfo.Name))
		}
		// Unexpected HTTP error
		return nil, err
	}

	// GetRemoteTags gets all tags & corresponding image IDs for a repo, returned in a map
	logrus.Debugf("Retrieving the tag list")
	tagsMap, err = mf.session.GetRemoteTags(repoData.Endpoints, mf.repoInfo.Name)
	if err != nil {
		logrus.Errorf("unable to get remote tags: %s", err)
		return nil, err
	}

	tagged, isTagged := ref.(reference.NamedTagged)
	if !isTagged {
		logrus.Errorf("No tag in image name! Christy messed up.")
		return nil, fmt.Errorf("fws: No tag in image name")
	}
	tag := tagged.Tag()
	tagID, err := mf.session.GetRemoteTag(repoData.Endpoints, mf.repoInfo.Name, tag)
	if err == registry.ErrRepoNotFound {
		return nil, fmt.Errorf("Tag %s not found in repository %s", tag, mf.repoInfo.Name.Name())
	}
	if err != nil {
		logrus.Errorf("unable to get remote tags: %s", err)
		return nil, err
	}
	tagsMap[tagged.Tag()] = tagID

	// Pull the tags from the tag/imgID map:
	tagList := []string{}
	for tag := range tagsMap {
		tagList = append(tagList, tag)
	}

	img := repoData.ImgList[tagID]

	for _, ep := range mf.repoInfo.Index.Mirrors {
		if pulledImg, err = mf.pullImageJSON(img.ID, ep); err != nil {
			// Don't report errors when pulling from mirrors.
			logrus.Debugf("Error pulling image json of %s:%s, mirror: %s, %s", mf.repoInfo.Name.Name(), img.Tag, ep, err)
			continue
		}
		break
	}
	if pulledImg == nil {
		for _, ep := range repoData.Endpoints {
			if pulledImg, err = mf.pullImageJSON(img.ID, ep); err != nil {
				// It's not ideal that only the last error is returned, it would be better to concatenate the errors.
				logrus.Infof("Error pulling image json of %s:%s, endpoint: %s, %v", mf.repoInfo.Name.Name(), img.Tag, ep, err)
				continue
			}
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("Error pulling image (%s) from %s, %v", img.Tag, mf.repoInfo.Name.Name(), err)
	}
	if pulledImg == nil {
		return nil, fmt.Errorf("No such image %s:%s", mf.repoInfo.Name.Name(), tag)
	}

	imageInsp := makeImgManifestInspect(ref.String(), pulledImg, tag, manifestInfo{}, schema1.MediaTypeManifest, tagList)
	imageList = append(imageList, *imageInsp)
	return imageList, nil
}

func (mf *v1ManifestFetcher) pullImageJSON(imgID, endpoint string) (*image.Image, error) {
	imgJSON, _, err := mf.session.GetRemoteImageJSON(imgID, endpoint)
	if err != nil {
		return nil, err
	}
	h, err := v1.HistoryFromConfig(imgJSON, false)
	if err != nil {
		return nil, err
	}
	configRaw, err := makeRawConfigFromV1Config(imgJSON, image.NewRootFS(), []image.History{h})
	if err != nil {
		return nil, err
	}
	config, err := json.Marshal(configRaw)
	if err != nil {
		return nil, err
	}
	img, err := image.NewFromJSON(config)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func makeRawConfigFromV1Config(imageJSON []byte, rootfs *image.RootFS, history []image.History) (map[string]*json.RawMessage, error) {
	var dver struct {
		DockerVersion string `json:"docker_version"`
	}

	if err := json.Unmarshal(imageJSON, &dver); err != nil {
		return nil, err
	}

	// the Version pkg was removed, so find a better way to do this
	useFallback := false
	cPoint := [3]int{1, 8, 3}
	for i, dPoint := range strings.Split(dver.DockerVersion, ".") {
		if x, _ := strconv.Atoi(dPoint); x < cPoint[i] {
			useFallback = true
		}
	}

	if useFallback {
		var v1Image image.V1Image
		err := json.Unmarshal(imageJSON, &v1Image)
		if err != nil {
			return nil, err
		}
		imageJSON, err = json.Marshal(v1Image)
		if err != nil {
			return nil, err
		}
	}

	var c map[string]*json.RawMessage
	if err := json.Unmarshal(imageJSON, &c); err != nil {
		return nil, err
	}

	c["rootfs"] = rawJSON(rootfs)
	c["history"] = rawJSON(history)

	return c, nil
}

// shouldV2Fallback returns true if this error is a reason to fall back to v1.
func shouldV2Fallback(err errcode.Error) bool {
	switch err.Code {
	case errcode.ErrorCodeUnauthorized, v2.ErrorCodeManifestUnknown, v2.ErrorCodeNameUnknown:
		return true
	}
	return false
}

func rawJSON(value interface{}) *json.RawMessage {
	jsonval, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return (*json.RawMessage)(&jsonval)
}
