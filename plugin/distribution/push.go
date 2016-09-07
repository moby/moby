// +build experimental

package distribution

import (
	"crypto/sha256"
	"io"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/docker/api/types"
	dockerdist "github.com/docker/docker/distribution"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"golang.org/x/net/context"
)

// Push pushes a plugin to a registry.
func Push(name string, rs registry.Service, metaHeader http.Header, authConfig *types.AuthConfig, config io.ReadCloser, layers io.ReadCloser) (digest.Digest, error) {
	ref, err := reference.ParseNamed(name)
	if err != nil {
		return "", err
	}

	repoInfo, err := rs.ResolveRepository(ref)
	if err != nil {
		return "", err
	}

	if err := dockerdist.ValidateRepoName(repoInfo.Name()); err != nil {
		return "", err
	}

	endpoints, err := rs.LookupPushEndpoints(repoInfo.Hostname())
	if err != nil {
		return "", err
	}

	var confirmedV2 bool
	var repository distribution.Repository
	for _, endpoint := range endpoints {
		if confirmedV2 && endpoint.Version == registry.APIVersion1 {
			logrus.Debugf("Skipping v1 endpoint %s because v2 registry was detected", endpoint.URL)
			continue
		}
		repository, confirmedV2, err = dockerdist.NewV2Repository(context.Background(), repoInfo, endpoint, metaHeader, authConfig, "push", "pull")
		if err != nil {
			return "", err
		}
		if !confirmedV2 {
			return "", ErrUnsupportedRegistry
		}
		logrus.Debugf("Trying to push %s to %s %s", repoInfo.Name(), endpoint.URL, endpoint.Version)
		// This means that we found an endpoint. and we are ready to push
		break
	}

	// Returns a reference to the repository's blob service.
	blobs := repository.Blobs(context.Background())

	// Descriptor = {mediaType, size, digest}
	var descs []distribution.Descriptor

	for i, f := range []io.ReadCloser{config, layers} {
		bw, err := blobs.Create(context.Background())
		if err != nil {
			logrus.Debugf("Error in blobs.Create: %v", err)
			return "", err
		}
		h := sha256.New()
		r := io.TeeReader(f, h)
		_, err = io.Copy(bw, r)
		if err != nil {
			f.Close()
			logrus.Debugf("Error in io.Copy: %v", err)
			return "", err
		}
		f.Close()
		mt := schema2.MediaTypeLayer
		if i == 0 {
			mt = schema2.MediaTypePluginConfig
		}
		// Commit completes the write process to the BlobService.
		// The descriptor arg to Commit is called the "provisional" descriptor and
		// used for validation.
		// The returned descriptor should be the one used. Its called the "Canonical"
		// descriptor.
		desc, err := bw.Commit(context.Background(), distribution.Descriptor{
			MediaType: mt,
			// XXX: What about the Size?
			Digest: digest.NewDigest("sha256", h),
		})
		if err != nil {
			logrus.Debugf("Error in bw.Commit: %v", err)
			return "", err
		}
		// The canonical descriptor is set the mediatype again, just in case.
		// Don't touch the digest or the size here.
		desc.MediaType = mt
		logrus.Debugf("pushed blob: %s %s", desc.MediaType, desc.Digest)
		descs = append(descs, desc)
	}

	// XXX: schema2.Versioned needs a MediaType as well.
	// "application/vnd.docker.distribution.manifest.v2+json"
	m, err := schema2.FromStruct(schema2.Manifest{Versioned: schema2.SchemaVersion, Config: descs[0], Layers: descs[1:]})
	if err != nil {
		logrus.Debugf("error in schema2.FromStruct: %v", err)
		return "", err
	}

	msv, err := repository.Manifests(context.Background())
	if err != nil {
		logrus.Debugf("error in repository.Manifests: %v", err)
		return "", err
	}

	_, pl, err := m.Payload()
	if err != nil {
		logrus.Debugf("error in m.Payload: %v", err)
		return "", err
	}

	logrus.Debugf("Pushed manifest: %s", pl)

	tag := DefaultTag
	if tagged, ok := ref.(reference.NamedTagged); ok {
		tag = tagged.Tag()
	}

	return msv.Put(context.Background(), m, distribution.WithTag(tag))
}
