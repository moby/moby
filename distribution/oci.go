package distribution

import (
	"fmt"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema2"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func init() {
	// TODO: Remove this registration if distribution is included with OCI support

	ocischemaFunc := func(b []byte) (distribution.Manifest, distribution.Descriptor, error) {
		m := new(schema2.DeserializedManifest)
		err := m.UnmarshalJSON(b)
		if err != nil {
			return nil, distribution.Descriptor{}, err
		}

		dgst := digest.FromBytes(b)
		return m, distribution.Descriptor{Digest: dgst, Size: int64(len(b)), MediaType: ocispec.MediaTypeImageManifest}, err
	}
	err := distribution.RegisterManifestSchema(ocispec.MediaTypeImageManifest, ocischemaFunc)
	if err != nil {
		panic(fmt.Sprintf("Unable to register manifest: %s", err))
	}

	manifestListFunc := func(b []byte) (distribution.Manifest, distribution.Descriptor, error) {
		m := new(manifestlist.DeserializedManifestList)
		err := m.UnmarshalJSON(b)
		if err != nil {
			return nil, distribution.Descriptor{}, err
		}

		dgst := digest.FromBytes(b)
		return m, distribution.Descriptor{Digest: dgst, Size: int64(len(b)), MediaType: ocispec.MediaTypeImageIndex}, err
	}
	err = distribution.RegisterManifestSchema(ocispec.MediaTypeImageIndex, manifestListFunc)
	if err != nil {
		panic(fmt.Sprintf("Unable to register manifest: %s", err))
	}
}
