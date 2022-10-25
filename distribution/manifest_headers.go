package distribution

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/sirupsen/logrus"
	"net/http"
)

const dockerManifestTagHeader = "Docker-Manifest-Tag"

// distributionRepositoryWithManifestInfo is a distribution.Repository implementation wrapper which keeps track on the
// manifest information (specifically the tag) being pushed / pulled. It also acts as transport.RequestModifier to
// modify requests, adding a header with the manifest tag.
type distributionRepositoryWithManifestInfo struct {
	distribution.Repository
	manifestInfo struct {
		tag string
	}
}

var _ distribution.Repository = (*distributionRepositoryWithManifestInfo)(nil)
var _ transport.RequestModifier = (*distributionRepositoryWithManifestInfo)(nil)

func (r *distributionRepositoryWithManifestInfo) ModifyRequest(req *http.Request) error {
	logrus.Tracef("distributionRepositoryWithManifestInfo.ModifyRequest: %s %s", req.Method, req.URL)
	info := r.manifestInfo
	if info.tag != "" {
		logrus.Tracef("distributionRepositoryWithManifestInfo: Setting manifest tag header - %s: %s", dockerManifestTagHeader, info.tag)
		req.Header.Set(dockerManifestTagHeader, info.tag)
	}
	return nil
}

// update the manifest info kept by this instance according to the given named ref.
func (r *distributionRepositoryWithManifestInfo) update(ref reference.Named) {
	if tagged, ok := ref.(reference.Tagged); ok {
		r.manifestInfo.tag = tagged.Tag()
		logrus.Tracef("distributionRepositoryWithManifestInfo: updated tag='%s' (from ref: %#v)", tagged.Tag(), ref)
	}
}

// updateRepoWithManifestInfo safely calls distributionRepositoryWithManifestInfo.update if repo is a
// distributionRepositoryWithManifestInfo
func updateRepoWithManifestInfo(repo distribution.Repository, ref reference.Named) {
	if r, ok := repo.(*distributionRepositoryWithManifestInfo); ok {
		r.update(ref)
	}
}

// metaHeadersWithManifestTagHeader returns a copy of the given meta headers map the 'Docker-Manifest-Tag' header with
// the tag value if the given ref is tagged, otherwise returns the original map.
func metaHeadersWithManifestTagHeader(metaHeaders map[string][]string, ref reference.Named) map[string][]string {
	tag, tagged := ref.(reference.NamedTagged)
	if !tagged || len(tag.Tag()) == 0 {
		logrus.Debugf("metaHeadersWithManifestTagHeader: ref is not tagged: %#v; skip adding metadata tag header", ref)
		return metaHeaders
	}
	// Copy the meta headers from the config
	newMetaHeaders := make(map[string][]string, len(metaHeaders))
	for k, v := range metaHeaders {
		newMetaHeaders[k] = v
	}

	logrus.Debugf("metaHeadersWithManifestTagHeader: Setting meta header: %s: %s", dockerManifestTagHeader, tag.Tag())
	newMetaHeaders[dockerManifestTagHeader] = []string{tag.Tag()}

	return newMetaHeaders
}
