package containerimage

import (
	"encoding/json"
	"testing"

	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/moby/buildkit/solver/pb"
	bkcontainerimage "github.com/moby/buildkit/source/containerimage"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/internal/layer"
	refstore "github.com/moby/moby/v2/daemon/internal/refstore"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

func TestRegistryIdentifierImageChecksum(t *testing.T) {
	checksum := digest.FromString("test")

	id, err := (&Source{}).registryIdentifier("docker.io/library/alpine:latest", map[string]string{
		pb.AttrImageChecksum: checksum.String(),
	}, nil)
	assert.NilError(t, err)

	assert.Equal(t, id.(*bkcontainerimage.ImageIdentifier).Checksum, checksum)
}

func TestRegistryIdentifierInvalidImageChecksum(t *testing.T) {
	_, err := (&Source{}).registryIdentifier("docker.io/library/alpine:latest", map[string]string{
		pb.AttrImageChecksum: "invalid",
	}, nil)
	assert.ErrorContains(t, err, "invalid image checksum")
}

func TestCacheKeyRemoteUsesManifest(t *testing.T) {
	config := imageConfig(t, digest.FromString("layer"))
	manifestDigest := digest.FromString("manifest")
	id, err := bkcontainerimage.NewImageIdentifier("docker.io/library/alpine:latest")
	assert.NilError(t, err)
	p := &puller{
		src:      id,
		desc:     ocispec.Descriptor{Digest: manifestDigest},
		config:   config,
		platform: platforms.DefaultSpec(),
	}
	p.resolveLocalOnce.Do(func() {})

	key, pin, _, done, err := p.CacheKey(t.Context(), nil, 0)
	assert.NilError(t, err)
	expected, err := p.mainManifestKey(p.platform)
	assert.NilError(t, err)
	assert.Equal(t, key, expected.String())
	assert.Equal(t, pin, manifestDigest.String())
	assert.Assert(t, done)
}

func TestCacheKeyVerifiedLocalUsesChainID(t *testing.T) {
	diffID := digest.FromString("layer")
	id, err := bkcontainerimage.NewImageIdentifier("docker.io/library/alpine:latest")
	assert.NilError(t, err)
	p := &puller{
		verifiedLocal: true,
		src:           id,
		config:        imageConfig(t, diffID),
		platform:      platforms.DefaultSpec(),
	}
	p.resolveLocalOnce.Do(func() {})

	key, pin, _, done, err := p.CacheKey(t.Context(), nil, 0)
	assert.NilError(t, err)
	expected := verifiedLocalCacheKey(p.config).String()
	assert.Equal(t, key, expected)
	assert.Equal(t, pin, expected)
	assert.Assert(t, key != diffID.String(), "verified local cache key must not collide with legacy rootfs keys")
	assert.Assert(t, done)
}

func TestCacheKeyVerifiedLocalWithManifest(t *testing.T) {
	diffID := digest.FromString("layer")
	manifestDigest := digest.FromString("manifest")
	id, err := bkcontainerimage.NewImageIdentifier("docker.io/library/alpine:latest")
	assert.NilError(t, err)
	p := &puller{
		verifiedLocal: true,
		src:           id,
		desc:          ocispec.Descriptor{Digest: manifestDigest},
		config:        imageConfig(t, diffID),
		platform:      platforms.DefaultSpec(),
	}
	p.resolveLocalOnce.Do(func() {})

	key, pin, _, done, err := p.CacheKey(t.Context(), nil, 0)
	assert.NilError(t, err)
	expectedManifest, err := p.mainManifestKey(p.platform)
	assert.NilError(t, err)
	assert.Equal(t, key, expectedManifest.String())
	assert.Equal(t, pin, manifestDigest.String())
	assert.Assert(t, !done)

	key, pin, _, done, err = p.CacheKey(t.Context(), nil, 1)
	assert.NilError(t, err)
	expectedLocal := verifiedLocalCacheKey(p.config).String()
	assert.Equal(t, key, expectedLocal)
	assert.Equal(t, pin, expectedLocal)
	assert.Assert(t, done)
}

func TestResolveLocalRequiresLayerStoreRootFS(t *testing.T) {
	diffIDs := []digest.Digest{digest.FromString("lower"), digest.FromString("upper")}
	config := imageConfig(t, diffIDs...)
	img, err := image.NewFromJSON(config)
	assert.NilError(t, err)
	imageID := digest.FromBytes(config)
	sourceID, err := bkcontainerimage.NewImageIdentifier("docker.io/library/alpine:latest")
	assert.NilError(t, err)

	for _, tc := range []struct {
		name     string
		layerErr error
		verified bool
	}{
		{name: "rootfs present", verified: true},
		{name: "rootfs missing", layerErr: layer.ErrLayerDoesNotExist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storedLayer := &localLayer{chainID: img.RootFS.ChainID()}
			layerStore := &localLayerStore{result: storedLayer, err: tc.layerErr}
			p := &puller{
				is: &Source{SourceOpt: SourceOpt{
					ReferenceStore: &localReferenceStore{id: imageID},
					ImageStore:     &localImageStore{img: img},
					LayerStore:     layerStore,
				}},
				src:      sourceID,
				platform: platforms.DefaultSpec(),
			}

			p.resolveLocal()

			assert.Equal(t, p.verifiedLocal, tc.verified)
			assert.Equal(t, layerStore.requested, img.RootFS.ChainID())
			assert.Assert(t, layerStore.requested != diffIDs[0])
			assert.Assert(t, layerStore.requested != diffIDs[1])
			if tc.verified {
				assert.DeepEqual(t, p.config, config)
				assert.Equal(t, layerStore.released, layer.Layer(storedLayer))
			} else {
				assert.Assert(t, p.config == nil)
				assert.Assert(t, layerStore.released == nil)
			}
		})
	}
}

func imageConfig(t *testing.T, diffIDs ...digest.Digest) []byte {
	t.Helper()
	platform := platforms.DefaultSpec()
	dt, err := json.Marshal(ocispec.Image{
		Platform: platform,
		RootFS: ocispec.RootFS{
			Type:    "layers",
			DiffIDs: diffIDs,
		},
	})
	assert.NilError(t, err)
	return dt
}

type localReferenceStore struct {
	refstore.Store
	id digest.Digest
}

func (s *localReferenceStore) Get(reference.Named) (digest.Digest, error) {
	return s.id, nil
}

type localImageStore struct {
	image.Store
	img *image.Image
}

func (s *localImageStore) Get(image.ID) (*image.Image, error) {
	return s.img, nil
}

type localLayerStore struct {
	layer.Store
	err       error
	result    layer.Layer
	requested layer.ChainID
	released  layer.Layer
}

func (s *localLayerStore) Get(chainID layer.ChainID) (layer.Layer, error) {
	s.requested = chainID
	return s.result, s.err
}

func (s *localLayerStore) Release(l layer.Layer) ([]layer.Metadata, error) {
	s.released = l
	return nil, nil
}

type localLayer struct {
	layer.Layer
	chainID layer.ChainID
}

func (l *localLayer) ChainID() layer.ChainID {
	return l.chainID
}
