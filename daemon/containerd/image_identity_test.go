package containerd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/metadata"
	"github.com/containerd/containerd/v2/pkg/labels"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/plugins/content/local"
	"github.com/containerd/platforms"
	imagetypes "github.com/moby/moby/api/types/image"
	"github.com/moby/moby/v2/daemon/containerd/identitycache"
	"github.com/moby/moby/v2/daemon/internal/builder-next/exporter"
	"github.com/moby/moby/v2/internal/testutil/specialimage"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	bolt "go.etcd.io/bbolt"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageIdentityCacheRoundTripDeepCopy(t *testing.T) {
	ctx := namespaces.WithNamespace(t.Context(), "testing-"+t.Name())
	service := &ImageService{content: newWritableContentStore(ctx, t)}

	key := imageIdentityCacheKey("img", "best", "linux/amd64")
	now := time.Now().UTC()
	signature := &imagetypes.SignatureIdentity{
		Name: "signature",
		Timestamps: []imagetypes.SignatureTimestamp{
			{
				Type:      imagetypes.SignatureTimestampTlog,
				URI:       "https://example.test/tlog",
				Timestamp: now,
			},
		},
		Warnings: []string{"warning"},
		Signer: &imagetypes.SignerIdentity{
			CertificateIssuer: "issuer",
		},
	}

	assert.NilError(t, service.updateImageIdentityCache(ctx, key, signature, time.Minute))

	// mutate source after storing to verify cache stores an isolated copy
	signature.Timestamps[0].URI = "mutated-source"
	signature.Warnings[0] = "mutated-source"
	signature.Signer.CertificateIssuer = "mutated-source"

	first, ok, err := service.imageSignatureIdentityFromCache(ctx, key)
	assert.NilError(t, err)
	assert.Check(t, ok)
	if first == nil {
		t.Fatal("expected cached signature identity")
	}
	if first.Signer == nil {
		t.Fatal("expected cached signature signer")
	}
	assert.Check(t, is.Equal(first.Timestamps[0].URI, "https://example.test/tlog"))
	assert.Check(t, is.Equal(first.Warnings[0], "warning"))
	assert.Check(t, is.Equal(first.Signer.CertificateIssuer, "issuer"))

	// mutate fetched value to verify reads also return isolated copies
	first.Timestamps[0].URI = "mutated-read"
	first.Warnings[0] = "mutated-read"
	first.Signer.CertificateIssuer = "mutated-read"

	second, ok, err := service.imageSignatureIdentityFromCache(ctx, key)
	assert.NilError(t, err)
	assert.Check(t, ok)
	if second == nil {
		t.Fatal("expected cached signature identity on second read")
	}
	if second.Signer == nil {
		t.Fatal("expected cached signature signer on second read")
	}
	assert.Check(t, is.Equal(second.Timestamps[0].URI, "https://example.test/tlog"))
	assert.Check(t, is.Equal(second.Warnings[0], "warning"))
	assert.Check(t, is.Equal(second.Signer.CertificateIssuer, "issuer"))
}

func TestImageIdentityCacheExpiredEntryIsDropped(t *testing.T) {
	ctx := namespaces.WithNamespace(t.Context(), "testing-"+t.Name())
	service := &ImageService{content: newWritableContentStore(ctx, t)}

	key := imageIdentityCacheKey("img", "best", "linux/amd64")
	service.identity.cache = map[string]imageIdentityCacheEntry{
		key: {
			CachedAt:  time.Now().UTC().Add(-2 * time.Minute),
			ExpiresAt: time.Now().UTC().Add(-1 * time.Minute),
			Signature: &imagetypes.SignatureIdentity{Name: "expired"},
		},
	}

	res, ok, err := service.imageSignatureIdentityFromCache(ctx, key)
	assert.NilError(t, err)
	assert.Check(t, !ok)
	assert.Check(t, res == nil)
	assert.Check(t, is.Len(service.identity.cache, 0))
}

func TestImageIdentityCacheZeroTTLNoop(t *testing.T) {
	ctx := namespaces.WithNamespace(t.Context(), "testing-"+t.Name())
	service := &ImageService{content: newWritableContentStore(ctx, t)}

	key := imageIdentityCacheKey("img", "best", "linux/amd64")
	assert.NilError(t, service.updateImageIdentityCache(ctx, key, &imagetypes.SignatureIdentity{Name: "sig"}, 0))
	assert.Check(t, is.Len(service.identity.cache, 0))
}

func TestImageIdentityCachePruneRemovesExpiredEntries(t *testing.T) {
	now := time.Now().UTC()
	entries := map[string]imageIdentityCacheEntry{
		"expired": {
			CachedAt:  now.Add(-2 * time.Hour),
			ExpiresAt: now.Add(-time.Second),
			Signature: &imagetypes.SignatureIdentity{Name: "expired"},
		},
		"fresh": {
			CachedAt:  now.Add(-time.Minute),
			ExpiresAt: now.Add(time.Hour),
			Signature: &imagetypes.SignatureIdentity{Name: "fresh"},
		},
	}

	pruneImageIdentityCacheEntries(entries, now)

	assert.Check(t, is.Len(entries, 1))
	_, expiredExists := entries["expired"]
	assert.Check(t, !expiredExists)
	_, freshExists := entries["fresh"]
	assert.Check(t, freshExists)
}

func TestImageIdentityBestMatchNil(t *testing.T) {
	d, p := imageIdentityBestMatch(nil)
	assert.Check(t, is.Equal(d, ""))
	assert.Check(t, is.Equal(p, ""))

	d, p = imageIdentityBestMatch(&multiPlatformSummary{})
	assert.Check(t, is.Equal(d, ""))
	assert.Check(t, is.Equal(p, ""))
}

func TestImageIdentityCacheKey(t *testing.T) {
	got := imageIdentityCacheKey("sha256:image", "sha256:best", "linux/amd64")
	assert.Check(t, is.Equal(got, "sha256:image|sha256:best|linux/amd64"))
}

func TestCloneSignatureIdentityNil(t *testing.T) {
	var sig *imagetypes.SignatureIdentity
	assert.Check(t, cloneSignatureIdentity(sig) == nil)
}

func TestImageIdentityCacheRoundTripNilSignature(t *testing.T) {
	ctx := namespaces.WithNamespace(t.Context(), "testing-"+t.Name())
	service := &ImageService{content: newWritableContentStore(ctx, t)}

	key := imageIdentityCacheKey("img", "best", "linux/amd64")
	assert.NilError(t, service.updateImageIdentityCache(ctx, key, nil, time.Minute))
	res, ok, err := service.imageSignatureIdentityFromCache(ctx, key)
	assert.NilError(t, err)
	assert.Check(t, ok)
	assert.Check(t, res == nil)
}

func TestImageIdentityCachePersistsAcrossRestart(t *testing.T) {
	ctx := namespaces.WithNamespace(t.Context(), "testing-"+t.Name())
	root := t.TempDir()
	backendA, err := identitycache.NewBoltDBBackend(root)
	assert.NilError(t, err)
	defer backendA.Close()
	serviceA := &ImageService{
		identity: imageIdentityState{
			cache:      map[string]imageIdentityCacheEntry{},
			cacheStore: backendA,
		},
	}

	key := imageIdentityCacheKey("img", "best", "linux/amd64")
	assert.NilError(t, serviceA.updateImageIdentityCache(ctx, key, &imagetypes.SignatureIdentity{Name: "persisted"}, time.Minute))
	assert.NilError(t, backendA.Close())

	backendB, err := identitycache.NewBoltDBBackend(root)
	assert.NilError(t, err)
	defer backendB.Close()
	serviceB := &ImageService{
		identity: imageIdentityState{
			cache:      map[string]imageIdentityCacheEntry{},
			cacheStore: backendB,
		},
	}

	res, ok, err := serviceB.imageSignatureIdentityFromCache(ctx, key)
	assert.NilError(t, err)
	assert.Check(t, ok)
	if res == nil {
		t.Fatal("expected cached signature identity after restart")
	}
	assert.Check(t, is.Equal(res.Name, "persisted"))
}

func TestImageIdentityCacheKeysToRefresh(t *testing.T) {
	ctx := namespaces.WithNamespace(t.Context(), "testing-"+t.Name())
	now := time.Now().UTC()

	service := &ImageService{
		identity: imageIdentityState{
			cache: map[string]imageIdentityCacheEntry{
				"expired": {
					CachedAt:  now.Add(-2 * time.Hour),
					ExpiresAt: now.Add(-time.Minute),
				},
				"near": {
					CachedAt:  now.Add(-time.Hour),
					ExpiresAt: now.Add(imageIdentityRefreshAhead / 2),
				},
				"fresh": {
					CachedAt:  now.Add(-time.Minute),
					ExpiresAt: now.Add(imageIdentityRefreshAhead + time.Hour),
				},
			},
		},
	}

	keys, err := service.imageIdentityCacheKeysToRefresh(ctx, now)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(keys, []string{"expired", "near"}))
}

func TestRefreshImageIdentityCacheUpdatesExpiredPersistedEntry(t *testing.T) {
	ctx := namespaces.WithNamespace(t.Context(), "testing-"+t.Name())

	blobsDir := t.TempDir()
	idx, err := specialimage.MultiLayer(blobsDir)
	assert.NilError(t, err)

	cs := &blobsDirContentStore{blobs: filepath.Join(blobsDir, "blobs/sha256")}
	service := fakeImageService(t, ctx, cs)

	img, err := service.images.Create(ctx, imagesFromIndex(idx)[0])
	assert.NilError(t, err)

	backend, err := identitycache.NewBoltDBBackend(t.TempDir())
	assert.NilError(t, err)
	defer backend.Close()

	service.identity.cacheStore = backend
	service.identity.cache = map[string]imageIdentityCacheEntry{}

	multi, err := service.multiPlatformSummary(ctx, img, matchAnyWithPreference(platforms.Default(), nil))
	assert.NilError(t, err)
	if multi.Best == nil {
		t.Fatal("expected best manifest for test image")
	}

	key := imageIdentityCacheKey(img.Target.Digest.String(), multi.Best.Target().Digest.String(), platforms.FormatAll(multi.BestPlatform))
	now := time.Now().UTC()
	assert.NilError(t, backend.Store(ctx, key, imageIdentityCacheEntry{
		CachedAt:  now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Minute),
	}, now))

	_, ok, err := backend.Load(ctx, key, now)
	assert.NilError(t, err)
	assert.Check(t, !ok)

	service.refreshImageIdentityCache(ctx, now)

	refreshed, ok, err := backend.Load(ctx, key, now)
	assert.NilError(t, err)
	assert.Check(t, ok)
	assert.Check(t, refreshed.ExpiresAt.After(now))
}

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "timeout network error" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return false }

func TestSignatureVerificationErrorIsTransient(t *testing.T) {
	t.Run("deadline exceeded", func(t *testing.T) {
		assert.Check(t, signatureVerificationErrorIsTransient(context.DeadlineExceeded))
	})
	t.Run("canceled", func(t *testing.T) {
		assert.Check(t, signatureVerificationErrorIsTransient(context.Canceled))
	})
	t.Run("timeout net error", func(t *testing.T) {
		var err error = timeoutNetError{}
		assert.Check(t, signatureVerificationErrorIsTransient(err))
	})
	t.Run("dns error", func(t *testing.T) {
		assert.Check(t, signatureVerificationErrorIsTransient(&net.DNSError{Err: "no such host", Name: "example.invalid"}))
	})
	t.Run("url wrapped timeout", func(t *testing.T) {
		err := &url.Error{
			Op:  "Get",
			URL: "https://example.invalid",
			Err: timeoutNetError{},
		}
		assert.Check(t, signatureVerificationErrorIsTransient(err))
	})
	t.Run("non transient", func(t *testing.T) {
		assert.Check(t, !signatureVerificationErrorIsTransient(errors.New("permanent failure")))
	})
}

func TestSignatureVerificationMessageIsTransient(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{name: "deadline", msg: "context deadline exceeded", want: true},
		{name: "io timeout", msg: "i/o timeout", want: true},
		{name: "tls timeout", msg: "tls handshake timeout", want: true},
		{name: "dns", msg: "no such host", want: true},
		{name: "dial tcp", msg: "dial tcp 127.0.0.1:443", want: true},
		{name: "uppercase still matches", msg: "NETWORK IS UNREACHABLE", want: true},
		{name: "non transient", msg: "invalid signature", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, is.Equal(signatureVerificationMessageIsTransient(tc.msg), tc.want))
		})
	}
}

func TestImageIdentityFromLabels(t *testing.T) {
	createdB := time.Date(2024, 7, 20, 0, 0, 0, 0, time.UTC)
	createdA := time.Date(2024, 7, 21, 0, 0, 0, 0, time.UTC)

	buildAdt, err := json.Marshal(exporter.BuildRefLabelValue{CreatedAt: &createdB})
	assert.NilError(t, err)
	buildBdt, err := json.Marshal(exporter.BuildRefLabelValue{CreatedAt: &createdA})
	assert.NilError(t, err)

	identity := imageIdentityFromLabels(t.Context(), map[string]string{
		exporter.BuildRefLabel + "z-ref":              string(buildBdt),
		exporter.BuildRefLabel + "a-ref":              string(buildAdt),
		exporter.BuildRefLabel + "bad":                "{not-json}",
		labels.LabelDistributionSource + ".docker.io": "library/alpine,library/alpine,invalid//repo",
		labels.LabelDistributionSource + ".ghcr.io":   "myorg/myimage",
	})

	assert.Check(t, is.Len(identity.Build, 2))
	assert.Check(t, is.Equal(identity.Build[0].Ref, "a-ref"))
	assert.Check(t, is.Equal(identity.Build[1].Ref, "z-ref"))
	assert.Check(t, is.Equal(identity.Build[0].CreatedAt, createdB))
	assert.Check(t, is.Equal(identity.Build[1].CreatedAt, createdA))

	repos := make([]string, 0, len(identity.Pull))
	for _, pull := range identity.Pull {
		repos = append(repos, pull.Repository)
	}
	sort.Strings(repos)
	assert.Check(t, is.DeepEqual(repos, []string{
		"docker.io/library/alpine",
		"ghcr.io/myorg/myimage",
	}))
}

func TestImageIdentityReturnsNilWhenNoData(t *testing.T) {
	ctx := namespaces.WithNamespace(t.Context(), "testing-"+t.Name())
	service := &ImageService{content: newWritableContentStore(ctx, t)}

	dgst := writeTestBlob(ctx, t, service.content, []byte("root"), nil)
	got, err := service.imageIdentity(ctx, ocispec.Descriptor{Digest: dgst}, nil)
	assert.NilError(t, err)
	assert.Check(t, got == nil)
}

func TestImageIdentityFromCacheOnlyDoesNotPopulateCache(t *testing.T) {
	ctx := namespaces.WithNamespace(t.Context(), "testing-"+t.Name())
	service := &ImageService{content: newWritableContentStore(ctx, t)}

	dgst := writeTestBlob(ctx, t, service.content, []byte("root"), nil)
	desc := ocispec.Descriptor{Digest: dgst}

	got, err := service.imageIdentityFromCache(ctx, desc, nil)
	assert.NilError(t, err)
	assert.Check(t, got == nil)
	assert.Check(t, is.Len(service.identity.cache, 0))

	got, err = service.imageIdentity(ctx, desc, nil)
	assert.NilError(t, err)
	assert.Check(t, got == nil)
	assert.Check(t, is.Len(service.identity.cache, 1))
}

func newWritableContentStore(ctx context.Context, t testing.TB) content.Store {
	t.Helper()

	dir := t.TempDir()
	bdb, err := bolt.Open(filepath.Join(dir, "metadata.db"), 0o600, &bolt.Options{})
	assert.NilError(t, err)
	t.Cleanup(func() { bdb.Close() })

	cs, err := local.NewStore(filepath.Join(dir, "blobs"))
	assert.NilError(t, err)

	mdb := metadata.NewDB(bdb, cs, nil)
	assert.NilError(t, mdb.Init(ctx))

	return mdb.ContentStore()
}

func writeTestBlob(ctx context.Context, t testing.TB, cs content.Store, payload []byte, lbls map[string]string) digest.Digest {
	t.Helper()

	dgst := digest.FromBytes(payload)
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageConfig,
		Digest:    dgst,
		Size:      int64(len(payload)),
	}
	var opts []content.Opt
	if lbls != nil {
		opts = append(opts, content.WithLabels(lbls))
	}
	err := content.WriteBlob(ctx, cs, dgst.String(), bytes.NewReader(payload), desc, opts...)
	assert.NilError(t, err)
	return dgst
}
