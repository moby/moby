package distribution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/plugins/content/local"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/ocischema"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/moby/moby/v2/internal/testutil/labelstore"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type mockManifestGetter struct {
	manifests map[digest.Digest]distribution.Manifest
	gets      int
}

func (m *mockManifestGetter) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	m.gets++
	manifest, ok := m.manifests[dgst]
	if !ok {
		return nil, distribution.ErrManifestUnknown{Tag: dgst.String()}
	}
	return manifest, nil
}

func (m *mockManifestGetter) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	_, ok := m.manifests[dgst]
	return ok, nil
}

type testingContentStoreWrapper struct {
	ContentStore
	errorOnWriter error
	errorOnCommit error
}

func (s *testingContentStoreWrapper) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	if s.errorOnWriter != nil {
		return nil, s.errorOnWriter
	}

	w, err := s.ContentStore.Writer(ctx, opts...)
	if err != nil {
		return nil, err
	}

	if s.errorOnCommit != nil {
		w = &testingContentWriterWrapper{w, s.errorOnCommit}
	}
	return w, nil
}

type testingContentWriterWrapper struct {
	content.Writer
	err error
}

func (w *testingContentWriterWrapper) Commit(ctx context.Context, size int64, dgst digest.Digest, opts ...content.Opt) error {
	if w.err != nil {
		// The contract for `Commit` is to always close.
		// Since this is returning early before hitting the real `Commit`, we should close it here.
		w.Close()
		return w.err
	}
	return w.Writer.Commit(ctx, size, dgst, opts...)
}

func TestManifestStore(t *testing.T) {
	ociManifest := &ocispec.Manifest{}
	serialized, err := json.Marshal(ociManifest)
	assert.NilError(t, err)
	dgst := digest.Canonical.FromBytes(serialized)

	setupTest := func(t *testing.T) (reference.Named, ocispec.Descriptor, *mockManifestGetter, *manifestStore, content.Store) {
		t.Helper()
		root := t.TempDir()

		cs, err := local.NewLabeledStore(root, &labelstore.InMemory{})
		assert.NilError(t, err)

		mg := &mockManifestGetter{manifests: make(map[digest.Digest]distribution.Manifest)}
		store := &manifestStore{local: cs, remote: mg}
		desc := ocispec.Descriptor{Digest: dgst, MediaType: ocispec.MediaTypeImageManifest, Size: int64(len(serialized))}

		ref, err := reference.Parse("foo/bar")
		assert.NilError(t, err)

		return ref.(reference.Named), desc, mg, store, cs
	}

	ctx := context.Background()

	m, _, err := distribution.UnmarshalManifest(ocispec.MediaTypeImageManifest, serialized)
	assert.NilError(t, err)

	writeManifest := func(t *testing.T, cs ContentStore, desc ocispec.Descriptor, opts ...content.Opt) {
		ingestKey := remotes.MakeRefKey(ctx, desc)
		w, err := cs.Writer(ctx, content.WithDescriptor(desc), content.WithRef(ingestKey))
		assert.NilError(t, err)
		defer func() {
			if err := w.Close(); err != nil {
				t.Log(err)
			}
			if t.Failed() {
				if err := cs.Abort(ctx, ingestKey); err != nil {
					t.Log(err)
				}
			}
		}()

		_, err = w.Write(serialized)
		assert.NilError(t, err)

		err = w.Commit(ctx, desc.Size, desc.Digest, opts...)
		assert.NilError(t, err)
	}

	// All tests should end up with no active ingest
	checkIngest := func(t *testing.T, cs content.Store, desc ocispec.Descriptor) {
		ingestKey := remotes.MakeRefKey(ctx, desc)
		_, err := cs.Status(ctx, ingestKey)
		assert.Check(t, cerrdefs.IsNotFound(err), err)
	}

	t.Run("no remote or local", func(t *testing.T) {
		ref, desc, _, store, cs := setupTest(t)

		_, err = store.Get(ctx, desc, ref)
		checkIngest(t, cs, desc)
		// This error is what our digest getter returns when it doesn't know about the manifest
		assert.Error(t, err, distribution.ErrManifestUnknown{Tag: dgst.String()}.Error())
	})

	t.Run("no local cache", func(t *testing.T) {
		ref, desc, mg, store, cs := setupTest(t)

		mg.manifests[desc.Digest] = m

		m2, err := store.Get(ctx, desc, ref)
		checkIngest(t, cs, desc)
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(m, m2, cmpopts.IgnoreUnexported(ocischema.DeserializedManifest{})))
		assert.Check(t, is.Equal(mg.gets, 1))

		i, err := cs.Info(ctx, desc.Digest)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(i.Digest, desc.Digest))

		distKey, distSource := makeDistributionSourceLabel(ref)
		assert.Check(t, hasDistributionSource(i.Labels[distKey], distSource))

		// Now check again, this should not hit the remote
		m2, err = store.Get(ctx, desc, ref)
		checkIngest(t, cs, desc)
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(m, m2, cmpopts.IgnoreUnexported(ocischema.DeserializedManifest{})))
		assert.Check(t, is.Equal(mg.gets, 1))

		t.Run("digested", func(t *testing.T) {
			ref, err := reference.WithDigest(ref, desc.Digest)
			assert.NilError(t, err)

			_, err = store.Get(ctx, desc, ref)
			assert.NilError(t, err)
		})
	})

	t.Run("with local cache", func(t *testing.T) {
		ref, desc, mg, store, cs := setupTest(t)

		// first add the manifest to the content store
		writeManifest(t, cs, desc)

		// now do the get
		m2, err := store.Get(ctx, desc, ref)
		checkIngest(t, cs, desc)
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(m, m2, cmpopts.IgnoreUnexported(ocischema.DeserializedManifest{})))
		assert.Check(t, is.Equal(mg.gets, 0))

		i, err := cs.Info(ctx, desc.Digest)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(i.Digest, desc.Digest))
	})

	// This is for the case of pull by digest where we don't know the media type of the manifest until it's actually pulled.
	t.Run("unknown media type", func(t *testing.T) {
		t.Run("no cache", func(t *testing.T) {
			ref, desc, mg, store, cs := setupTest(t)

			mg.manifests[desc.Digest] = m
			desc.MediaType = ""

			m2, err := store.Get(ctx, desc, ref)
			checkIngest(t, cs, desc)
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(m, m2, cmpopts.IgnoreUnexported(ocischema.DeserializedManifest{})))
			assert.Check(t, is.Equal(mg.gets, 1))
		})

		t.Run("with cache", func(t *testing.T) {
			t.Run("cached manifest has media type", func(t *testing.T) {
				ref, desc, mg, store, cs := setupTest(t)

				writeManifest(t, cs, desc)
				desc.MediaType = ""

				m2, err := store.Get(ctx, desc, ref)
				checkIngest(t, cs, desc)
				assert.NilError(t, err)
				assert.Check(t, is.DeepEqual(m, m2, cmpopts.IgnoreUnexported(ocischema.DeserializedManifest{})))
				assert.Check(t, is.Equal(mg.gets, 0))
			})

			t.Run("cached manifest has no media type", func(t *testing.T) {
				ref, desc, mg, store, cs := setupTest(t)

				desc.MediaType = ""
				writeManifest(t, cs, desc)

				m2, err := store.Get(ctx, desc, ref)
				checkIngest(t, cs, desc)
				assert.NilError(t, err)
				assert.Check(t, is.DeepEqual(m, m2, cmpopts.IgnoreUnexported(ocischema.DeserializedManifest{})))
				assert.Check(t, is.Equal(mg.gets, 0))
			})
		})
	})

	// Test that if there is an error with the content store, for whatever
	// reason, that doesn't stop us from getting the manifest.
	//
	// Also makes sure the ingests are aborted.
	t.Run("error persisting manifest", func(t *testing.T) {
		t.Run("error on writer", func(t *testing.T) {
			ref, desc, mg, store, cs := setupTest(t)
			mg.manifests[desc.Digest] = m

			csW := &testingContentStoreWrapper{ContentStore: store.local, errorOnWriter: errors.New("random error")}
			store.local = csW

			m2, err := store.Get(ctx, desc, ref)
			checkIngest(t, cs, desc)
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(m, m2, cmpopts.IgnoreUnexported(ocischema.DeserializedManifest{})))
			assert.Check(t, is.Equal(mg.gets, 1))

			_, err = cs.Info(ctx, desc.Digest)
			// Nothing here since we couldn't persist
			assert.Check(t, cerrdefs.IsNotFound(err), err)
		})

		t.Run("error on commit", func(t *testing.T) {
			ref, desc, mg, store, cs := setupTest(t)
			mg.manifests[desc.Digest] = m

			csW := &testingContentStoreWrapper{ContentStore: store.local, errorOnCommit: errors.New("random error")}
			store.local = csW

			m2, err := store.Get(ctx, desc, ref)
			checkIngest(t, cs, desc)
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(m, m2, cmpopts.IgnoreUnexported(ocischema.DeserializedManifest{})))
			assert.Check(t, is.Equal(mg.gets, 1))

			_, err = cs.Info(ctx, desc.Digest)
			// Nothing here since we couldn't persist
			assert.Check(t, cerrdefs.IsNotFound(err), err)
		})
	})
}

func TestDetectManifestBlobMediaType(t *testing.T) {
	type testCase struct {
		json     []byte
		expected string
	}
	cases := map[string]testCase{
		"mediaType is set":   {[]byte(`{"mediaType": "bananas"}`), "bananas"},
		"oci manifest":       {[]byte(`{"config": {}}`), ocispec.MediaTypeImageManifest},
		"oci index fallback": {[]byte(`{}`), ocispec.MediaTypeImageIndex},
		// Make sure we prefer mediaType
		"mediaType and config set":   {[]byte(`{"mediaType": "bananas", "config": {}}`), "bananas"},
		"mediaType and fsLayers set": {[]byte(`{"mediaType": "bananas", "fsLayers": []}`), "bananas"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mt, err := detectManifestBlobMediaType(tc.json)
			assert.NilError(t, err)
			assert.Equal(t, mt, tc.expected)
		})
	}
}

func TestDetectManifestBlobMediaTypeInvalid(t *testing.T) {
	type testCase struct {
		json     []byte
		expected string
	}
	cases := map[string]testCase{
		"schema 2 mediaType with manifests": {
			[]byte(`{"mediaType": "` + schema2.MediaTypeManifest + `","manifests":[]}`),
			`media-type: "application/vnd.docker.distribution.manifest.v2+json" should not have "manifests" or "fsLayers"`,
		},
		"schema 2 mediaType with fsLayers": {
			[]byte(`{"mediaType": "` + schema2.MediaTypeManifest + `","fsLayers":[]}`),
			`media-type: "application/vnd.docker.distribution.manifest.v2+json" should not have "manifests" or "fsLayers"`,
		},
		"oci manifest mediaType with manifests": {
			[]byte(`{"mediaType": "` + ocispec.MediaTypeImageManifest + `","manifests":[]}`),
			`media-type: "application/vnd.oci.image.manifest.v1+json" should not have "manifests" or "fsLayers"`,
		},
		"manifest list mediaType with fsLayers": {
			[]byte(`{"mediaType": "` + manifestlist.MediaTypeManifestList + `","fsLayers":[]}`),
			`media-type: "application/vnd.docker.distribution.manifest.list.v2+json" should not have "config", "layers", or "fsLayers"`,
		},
		"index mediaType with layers": {
			[]byte(`{"mediaType": "` + ocispec.MediaTypeImageIndex + `","layers":[]}`),
			`media-type: "application/vnd.oci.image.index.v1+json" should not have "config", "layers", or "fsLayers"`,
		},
		"index mediaType with config": {
			[]byte(`{"mediaType": "` + ocispec.MediaTypeImageIndex + `","config":{}}`),
			`media-type: "application/vnd.oci.image.index.v1+json" should not have "config", "layers", or "fsLayers"`,
		},
		"config and manifests": {
			[]byte(`{"config":{}, "manifests":[]}`),
			`media-type: cannot determine`,
		},
		"layers and manifests": {
			[]byte(`{"layers":[], "manifests":[]}`),
			`media-type: cannot determine`,
		},
		"layers and fsLayers": {
			[]byte(`{"layers":[], "fsLayers":[]}`),
			`media-type: cannot determine`,
		},
		"fsLayers and manifests": {
			[]byte(`{"fsLayers":[], "manifests":[]}`),
			`media-type: cannot determine`,
		},
		"config and fsLayers": {
			[]byte(`{"config":{}, "fsLayers":[]}`),
			`media-type: cannot determine`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mt, err := detectManifestBlobMediaType(tc.json)
			assert.Error(t, err, tc.expected)
			assert.Equal(t, mt, "")
		})
	}
}
