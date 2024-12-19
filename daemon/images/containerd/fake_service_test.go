package containerd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/snapshots"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/container"
	daemonevents "github.com/docker/docker/daemon/events"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

func fakeImageService(t testing.TB, ctx context.Context, cs content.Store) *ImageService {
	snapshotter := &testSnapshotterService{}

	mdb := newTestDB(ctx, t)

	snapshotters := map[string]snapshots.Snapshotter{
		containerd.DefaultSnapshotter: snapshotter,
	}

	service := &ImageService{
		images:              metadata.NewImageStore(mdb),
		containers:          container.NewMemoryStore(),
		content:             cs,
		eventsService:       daemonevents.New(),
		snapshotterServices: snapshotters,
		snapshotter:         containerd.DefaultSnapshotter,
	}

	// containerd.Image gets the services directly from containerd.Client
	// so we need to create a "fake" containerd.Client with the test services.
	c8dCli, err := containerd.New("", containerd.WithServices(
		containerd.WithImageStore(service.images),
		containerd.WithContentStore(cs),
		containerd.WithSnapshotters(snapshotters),
		containerd.WithLeasesService(noopLeasesManager{}),
	))
	assert.NilError(t, err)

	service.client = c8dCli
	return service
}

type noopLeasesManager struct{}

func (noopLeasesManager) Create(context.Context, ...leases.Opt) (leases.Lease, error) {
	return leases.Lease{}, nil
}

func (noopLeasesManager) Delete(context.Context, leases.Lease, ...leases.DeleteOpt) error {
	return nil
}

func (noopLeasesManager) List(context.Context, ...string) ([]leases.Lease, error) {
	return nil, nil
}

func (noopLeasesManager) AddResource(context.Context, leases.Lease, leases.Resource) error {
	return nil
}

func (noopLeasesManager) DeleteResource(context.Context, leases.Lease, leases.Resource) error {
	return nil
}

func (noopLeasesManager) ListResources(context.Context, leases.Lease) ([]leases.Resource, error) {
	return nil, nil
}

type blobsDirContentStore struct {
	blobs string
}

type fileReaderAt struct {
	*os.File
}

func (f *fileReaderAt) Size() int64 {
	fi, err := f.Stat()
	if err != nil {
		return -1
	}
	return fi.Size()
}

func (s *blobsDirContentStore) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	p := filepath.Join(s.blobs, desc.Digest.Encoded())
	r, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", cerrdefs.ErrNotFound, desc.Digest)
		}
		return nil, err
	}
	return &fileReaderAt{r}, nil
}

func (s *blobsDirContentStore) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	return nil, fmt.Errorf("read-only")
}

func (s *blobsDirContentStore) Status(ctx context.Context, _ string) (content.Status, error) {
	return content.Status{}, fmt.Errorf("not implemented")
}

func (s *blobsDirContentStore) Delete(ctx context.Context, dgst digest.Digest) error {
	p := filepath.Join(s.blobs, dgst.Encoded())
	return os.Remove(p)
}

func (s *blobsDirContentStore) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	return nil, nil
}

func (s *blobsDirContentStore) Abort(ctx context.Context, ref string) error {
	return fmt.Errorf("not implemented")
}

func (s *blobsDirContentStore) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	entries, err := os.ReadDir(s.blobs)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		d := digest.FromString(e.Name())
		if d == "" {
			continue
		}

		stat, err := e.Info()
		if err != nil {
			return err
		}

		if err := fn(content.Info{Digest: d, Size: stat.Size()}); err != nil {
			return err
		}
	}

	return nil
}

func (s *blobsDirContentStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	f, err := os.Open(filepath.Join(s.blobs, dgst.Encoded()))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return content.Info{}, fmt.Errorf("%w: %s", cerrdefs.ErrNotFound, dgst)
		}
		return content.Info{}, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return content.Info{}, err
	}

	return content.Info{
		Digest: dgst,
		Size:   stat.Size(),
	}, nil
}

func (s *blobsDirContentStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	return content.Info{}, fmt.Errorf("read-only")
}

// delayedStore is a content store wrapper that adds a constant delay to all
// operations in order to imitate gRPC overhead.
//
// The delay is constant to make the benchmark results more reproducible
// Since content store may be accessed concurrently random delay would be
// order-dependent.
type delayedStore struct {
	store    content.Store
	overhead time.Duration
}

func (s *delayedStore) delay() {
	time.Sleep(s.overhead)
}

func (s *delayedStore) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	s.delay()
	return s.store.ReaderAt(ctx, desc)
}

func (s *delayedStore) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	s.delay()
	return s.store.Writer(ctx, opts...)
}

func (s *delayedStore) Status(ctx context.Context, st string) (content.Status, error) {
	s.delay()
	return s.store.Status(ctx, st)
}

func (s *delayedStore) Delete(ctx context.Context, dgst digest.Digest) error {
	s.delay()
	return s.store.Delete(ctx, dgst)
}

func (s *delayedStore) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	s.delay()
	return s.store.ListStatuses(ctx, filters...)
}

func (s *delayedStore) Abort(ctx context.Context, ref string) error {
	s.delay()
	return s.store.Abort(ctx, ref)
}

func (s *delayedStore) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	s.delay()
	return s.store.Walk(ctx, fn, filters...)
}

func (s *delayedStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	s.delay()
	return s.store.Info(ctx, dgst)
}

func (s *delayedStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	s.delay()
	return s.store.Update(ctx, info, fieldpaths...)
}

type memoryLabelStore struct {
	mu     sync.Mutex
	labels map[digest.Digest]map[string]string
}

// Get returns all the labels for the given digest
func (s *memoryLabelStore) Get(dgst digest.Digest) (map[string]string, error) {
	s.mu.Lock()
	labels := s.labels[dgst]
	s.mu.Unlock()
	return labels, nil
}

// Set sets all the labels for a given digest
func (s *memoryLabelStore) Set(dgst digest.Digest, labels map[string]string) error {
	s.mu.Lock()
	if s.labels == nil {
		s.labels = make(map[digest.Digest]map[string]string)
	}
	s.labels[dgst] = labels
	s.mu.Unlock()
	return nil
}

// Update replaces the given labels for a digest,
// a key with an empty value removes a label.
func (s *memoryLabelStore) Update(dgst digest.Digest, update map[string]string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	labels, ok := s.labels[dgst]
	if !ok {
		labels = map[string]string{}
	}
	for k, v := range update {
		labels[k] = v
	}
	if s.labels == nil {
		s.labels = map[digest.Digest]map[string]string{}
	}
	s.labels[dgst] = labels

	return labels, nil
}
