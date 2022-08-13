package gha

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/moby/buildkit/cache/remotecache"
	v1 "github.com/moby/buildkit/cache/remotecache/v1"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/tracing"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	actionscache "github.com/tonistiigi/go-actions-cache"
	"golang.org/x/sync/errgroup"
)

func init() {
	actionscache.Log = logrus.Debugf
}

const (
	attrScope = "scope"
	attrToken = "token"
	attrURL   = "url"
	version   = "1"
)

type Config struct {
	Scope string
	URL   string
	Token string
}

func getConfig(attrs map[string]string) (*Config, error) {
	scope, ok := attrs[attrScope]
	if !ok {
		scope = "buildkit"
	}
	url, ok := attrs[attrURL]
	if !ok {
		return nil, errors.Errorf("url not set for github actions cache")
	}
	token, ok := attrs[attrToken]
	if !ok {
		return nil, errors.Errorf("token not set for github actions cache")
	}
	return &Config{
		Scope: scope,
		URL:   url,
		Token: token,
	}, nil
}

// ResolveCacheExporterFunc for Github actions cache exporter.
func ResolveCacheExporterFunc() remotecache.ResolveCacheExporterFunc {
	return func(ctx context.Context, g session.Group, attrs map[string]string) (remotecache.Exporter, error) {
		cfg, err := getConfig(attrs)
		if err != nil {
			return nil, err
		}
		return NewExporter(cfg)
	}
}

type exporter struct {
	solver.CacheExporterTarget
	chains *v1.CacheChains
	cache  *actionscache.Cache
	config *Config
}

func NewExporter(c *Config) (remotecache.Exporter, error) {
	cc := v1.NewCacheChains()
	cache, err := actionscache.New(c.Token, c.URL, actionscache.Opt{Client: tracing.DefaultClient})
	if err != nil {
		return nil, err
	}
	return &exporter{CacheExporterTarget: cc, chains: cc, cache: cache, config: c}, nil
}

func (ce *exporter) Config() remotecache.Config {
	return remotecache.Config{
		Compression: compression.New(compression.Default),
	}
}

func (ce *exporter) blobKey(dgst digest.Digest) string {
	return "buildkit-blob-" + version + "-" + dgst.String()
}

func (ce *exporter) indexKey() string {
	scope := ""
	for _, s := range ce.cache.Scopes() {
		if s.Permission&actionscache.PermissionWrite != 0 {
			scope = s.Scope
		}
	}
	scope = digest.FromBytes([]byte(scope)).Hex()[:8]
	return "index-" + ce.config.Scope + "-" + version + "-" + scope
}

func (ce *exporter) Finalize(ctx context.Context) (map[string]string, error) {
	// res := make(map[string]string)
	config, descs, err := ce.chains.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	// TODO: push parallel
	for i, l := range config.Layers {
		dgstPair, ok := descs[l.Blob]
		if !ok {
			return nil, errors.Errorf("missing blob %s", l.Blob)
		}
		if dgstPair.Descriptor.Annotations == nil {
			return nil, errors.Errorf("invalid descriptor without annotations")
		}
		var diffID digest.Digest
		v, ok := dgstPair.Descriptor.Annotations["containerd.io/uncompressed"]
		if !ok {
			return nil, errors.Errorf("invalid descriptor without uncompressed annotation")
		}
		dgst, err := digest.Parse(v)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse uncompressed annotation")
		}
		diffID = dgst

		key := ce.blobKey(dgstPair.Descriptor.Digest)
		b, err := ce.cache.Load(ctx, key)
		if err != nil {
			return nil, err
		}
		if b == nil {
			layerDone := oneOffProgress(ctx, fmt.Sprintf("writing layer %s", l.Blob))
			ra, err := dgstPair.Provider.ReaderAt(ctx, dgstPair.Descriptor)
			if err != nil {
				return nil, layerDone(err)
			}
			if err := ce.cache.Save(ctx, key, ra); err != nil {
				if !errors.Is(err, os.ErrExist) {
					return nil, layerDone(errors.Wrap(err, "error writing layer blob"))
				}
			}
			layerDone(nil)
		}
		la := &v1.LayerAnnotations{
			DiffID:    diffID,
			Size:      dgstPair.Descriptor.Size,
			MediaType: dgstPair.Descriptor.MediaType,
		}
		if v, ok := dgstPair.Descriptor.Annotations["buildkit/createdat"]; ok {
			var t time.Time
			if err := (&t).UnmarshalText([]byte(v)); err != nil {
				return nil, err
			}
			la.CreatedAt = t.UTC()
		}
		config.Layers[i].Annotations = la
	}

	dt, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	if err := ce.cache.SaveMutable(ctx, ce.indexKey(), 15*time.Second, func(old *actionscache.Entry) (actionscache.Blob, error) {
		return actionscache.NewBlob(dt), nil
	}); err != nil {
		return nil, err
	}

	return nil, nil
}

// ResolveCacheImporterFunc for Github actions cache importer.
func ResolveCacheImporterFunc() remotecache.ResolveCacheImporterFunc {
	return func(ctx context.Context, g session.Group, attrs map[string]string) (remotecache.Importer, ocispecs.Descriptor, error) {
		cfg, err := getConfig(attrs)
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		i, err := NewImporter(cfg)
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		return i, ocispecs.Descriptor{}, nil
	}
}

type importer struct {
	cache  *actionscache.Cache
	config *Config
}

func NewImporter(c *Config) (remotecache.Importer, error) {
	cache, err := actionscache.New(c.Token, c.URL, actionscache.Opt{Client: tracing.DefaultClient})
	if err != nil {
		return nil, err
	}
	return &importer{cache: cache, config: c}, nil
}

func (ci *importer) makeDescriptorProviderPair(l v1.CacheLayer) (*v1.DescriptorProviderPair, error) {
	if l.Annotations == nil {
		return nil, errors.Errorf("cache layer with missing annotations")
	}
	annotations := map[string]string{}
	if l.Annotations.DiffID == "" {
		return nil, errors.Errorf("cache layer with missing diffid")
	}
	annotations["containerd.io/uncompressed"] = l.Annotations.DiffID.String()
	if !l.Annotations.CreatedAt.IsZero() {
		txt, err := l.Annotations.CreatedAt.MarshalText()
		if err != nil {
			return nil, err
		}
		annotations["buildkit/createdat"] = string(txt)
	}
	desc := ocispecs.Descriptor{
		MediaType:   l.Annotations.MediaType,
		Digest:      l.Blob,
		Size:        l.Annotations.Size,
		Annotations: annotations,
	}
	return &v1.DescriptorProviderPair{
		Descriptor: desc,
		Provider:   &ciProvider{desc: desc, ci: ci},
	}, nil
}

func (ci *importer) loadScope(ctx context.Context, scope string) (*v1.CacheChains, error) {
	scope = digest.FromBytes([]byte(scope)).Hex()[:8]
	key := "index-" + ci.config.Scope + "-" + version + "-" + scope

	entry, err := ci.cache.Load(ctx, key)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return v1.NewCacheChains(), nil
	}

	// TODO: this buffer can be removed
	buf := &bytes.Buffer{}
	if err := entry.WriteTo(ctx, buf); err != nil {
		return nil, err
	}

	var config v1.CacheConfig
	if err := json.Unmarshal(buf.Bytes(), &config); err != nil {
		return nil, errors.WithStack(err)
	}

	allLayers := v1.DescriptorProvider{}

	for _, l := range config.Layers {
		dpp, err := ci.makeDescriptorProviderPair(l)
		if err != nil {
			return nil, err
		}
		allLayers[l.Blob] = *dpp
	}

	cc := v1.NewCacheChains()
	if err := v1.ParseConfig(config, allLayers, cc); err != nil {
		return nil, err
	}
	return cc, nil
}

func (ci *importer) Resolve(ctx context.Context, _ ocispecs.Descriptor, id string, w worker.Worker) (solver.CacheManager, error) {
	eg, ctx := errgroup.WithContext(ctx)
	ccs := make([]*v1.CacheChains, len(ci.cache.Scopes()))

	for i, s := range ci.cache.Scopes() {
		func(i int, scope string) {
			eg.Go(func() error {
				cc, err := ci.loadScope(ctx, scope)
				if err != nil {
					return err
				}
				ccs[i] = cc
				return nil
			})
		}(i, s.Scope)
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	cms := make([]solver.CacheManager, 0, len(ccs))

	for _, cc := range ccs {
		keysStorage, resultStorage, err := v1.NewCacheKeyStorage(cc, w)
		if err != nil {
			return nil, err
		}
		cms = append(cms, solver.NewCacheManager(ctx, id, keysStorage, resultStorage))
	}

	return solver.NewCombinedCacheManager(cms, nil), nil
}

type ciProvider struct {
	ci      *importer
	desc    ocispecs.Descriptor
	mu      sync.Mutex
	entries map[digest.Digest]*actionscache.Entry
}

func (p *ciProvider) CheckDescriptor(ctx context.Context, desc ocispecs.Descriptor) error {
	if desc.Digest != p.desc.Digest {
		return nil
	}

	_, err := p.loadEntry(ctx, desc)
	return err
}

func (p *ciProvider) loadEntry(ctx context.Context, desc ocispecs.Descriptor) (*actionscache.Entry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ce, ok := p.entries[desc.Digest]; ok {
		return ce, nil
	}
	key := "buildkit-blob-" + version + "-" + desc.Digest.String()
	ce, err := p.ci.cache.Load(ctx, key)
	if err != nil {
		return nil, err
	}
	if ce == nil {
		return nil, errors.Errorf("blob %s not found", desc.Digest)
	}
	if p.entries == nil {
		p.entries = make(map[digest.Digest]*actionscache.Entry)
	}
	p.entries[desc.Digest] = ce
	return ce, nil
}

func (p *ciProvider) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	ce, err := p.loadEntry(ctx, desc)
	if err != nil {
		return nil, err
	}
	rac := ce.Download(context.TODO())
	return &readerAt{ReaderAtCloser: rac, desc: desc}, nil
}

type readerAt struct {
	actionscache.ReaderAtCloser
	desc ocispecs.Descriptor
}

func (r *readerAt) Size() int64 {
	return r.desc.Size
}

func oneOffProgress(ctx context.Context, id string) func(err error) error {
	pw, _, _ := progress.NewFromContext(ctx)
	now := time.Now()
	st := progress.Status{
		Started: &now,
	}
	pw.Write(id, st)
	return func(err error) error {
		now := time.Now()
		st.Completed = &now
		pw.Write(id, st)
		pw.Close()
		return err
	}
}
