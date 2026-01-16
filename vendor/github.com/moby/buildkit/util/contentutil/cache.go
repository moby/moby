package contentutil

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/remotes"
	cerrdefs "github.com/containerd/errdefs"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func ReferrersProviderWithBuffer(p ReferrersProvider, buffer Buffer, name string) *ReferrersProviderBuffer {
	return &ReferrersProviderBuffer{
		p:     p,
		cache: buffer,
		name:  name,
	}
}

var _ ReferrersProvider = &ReferrersProviderBuffer{}

type ReferrersProviderBuffer struct {
	p     ReferrersProvider
	cache Buffer
	name  string

	mu    sync.Mutex
	blobs map[digest.Digest]ocispecs.Descriptor
	refs  map[digest.Digest][]ocispecs.Descriptor
}

func (p *ReferrersProviderBuffer) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	cw, err := content.OpenWriter(ctx, p.cache, content.WithDescriptor(desc), content.WithRef(desc.Digest.String()))
	if err != nil {
		if cerrdefs.IsAlreadyExists(err) {
			ra, err := p.cache.ReaderAt(ctx, desc)
			if err != nil {
				return nil, err
			}
			p.mu.Lock()
			if p.blobs == nil {
				p.blobs = make(map[digest.Digest]ocispecs.Descriptor)
			}
			p.blobs[desc.Digest] = desc
			p.mu.Unlock()
			return ra, nil
		}
		return nil, err
	}
	ra, err := p.p.ReaderAt(ctx, desc)
	if err != nil {
		cw.Close()
		return nil, err
	}
	if err := content.CopyReaderAt(cw, ra, ra.Size()); err != nil {
		cw.Close()
		return nil, err
	}
	if err := cw.Commit(ctx, desc.Size, desc.Digest); err != nil {
		cw.Close()
		return nil, err
	}
	ra, err = p.cache.ReaderAt(ctx, desc)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	if p.blobs == nil {
		p.blobs = make(map[digest.Digest]ocispecs.Descriptor)
	}
	p.blobs[desc.Digest] = desc
	p.mu.Unlock()
	return ra, nil
}

func (p *ReferrersProviderBuffer) FetchReferrers(ctx context.Context, dgst digest.Digest, opts ...remotes.FetchReferrersOpt) ([]ocispecs.Descriptor, error) {
	cfg := remotes.FetchReferrersConfig{}
	for _, o := range opts {
		if err := o(ctx, &cfg); err != nil {
			return nil, err
		}
	}

	info, err := p.cache.Info(ctx, dgst)
	if err == nil && len(info.Labels) != 0 {
		refs := []ocispecs.Descriptor{}
		for l, v := range info.Labels {
			if !strings.HasPrefix(l, "containerd.io/gc.ref.content.buildkit.refs.") {
				continue
			}
			dgst, err := digest.Parse(v)
			if err != nil {
				continue
			}
			dt, err := content.ReadBlob(ctx, p.cache, ocispecs.Descriptor{Digest: dgst})
			if err != nil {
				continue
			}
			desc := ocispecs.Descriptor{
				Digest:       dgst,
				Size:         int64(len(dt)),
				ArtifactType: readArtifactType(dt),
			}
			refs = append(refs, desc)
		}
		refs = filterRefs(refs, &cfg)
		if len(refs) > 0 {
			return refs, nil
		}
		v, ok := info.Labels["buildkit/refs.null"]
		if ok {
			for name := range strings.SplitSeq(v, ",") {
				if name == p.name {
					return nil, nil
				}
			}
		}
	}

	refs, err := p.p.FetchReferrers(ctx, dgst, opts...)
	if err != nil {
		return nil, err
	}
	refs = filterRefs(refs, &cfg)
	p.mu.Lock()
	if p.refs == nil {
		p.refs = make(map[digest.Digest][]ocispecs.Descriptor)
	}
	p.refs[dgst] = append(p.refs[dgst], refs...)
	p.mu.Unlock()

	return refs, nil
}

func (p *ReferrersProviderBuffer) SetGCLabels(ctx context.Context, root ocispecs.Descriptor) error {
	labels := map[string]string{}
	fieldpaths := []string{}

	p.mu.Lock()
	for _, desc := range p.blobs {
		shaPrefix := desc.Digest.Hex()[:12]
		key := "containerd.io/gc.ref.content.buildkit." + shaPrefix
		labels[key] = desc.Digest.String()
		fieldpaths = append(fieldpaths, "labels."+key)
	}
	p.mu.Unlock()

	_, err := p.cache.Update(ctx, content.Info{
		Digest: root.Digest,
		Labels: labels,
	}, fieldpaths...)
	if err != nil {
		return err
	}

	for dgst, refs := range p.refs {
		info, err := p.cache.Info(ctx, dgst)
		if err != nil {
			continue
		}
		labels := map[string]string{}
		fieldpaths := []string{}
		for _, ref := range refs {
			shaPrefix := ref.Digest.Hex()[:12]
			key := "containerd.io/gc.ref.content.buildkit.refs." + shaPrefix
			labels[key] = ref.Digest.String()
			fieldpaths = append(fieldpaths, "labels."+key)
		}
		if len(refs) == 0 {
			key := "buildkit/refs.null"
			labels[key] = addName(info.Labels[key], p.name)
			fieldpaths = append(fieldpaths, "labels."+key)
		}
		if len(labels) == 0 {
			continue
		}
		_, err = p.cache.Update(ctx, content.Info{
			Digest: dgst,
			Labels: labels,
		}, fieldpaths...)
		if err != nil {
			return err
		}
	}
	return nil
}

func filterRefs(refs []ocispecs.Descriptor, cfg *remotes.FetchReferrersConfig) []ocispecs.Descriptor {
	if len(cfg.ArtifactTypes) == 0 {
		return refs
	}
	out := []ocispecs.Descriptor{}
	for _, ref := range refs {
		if slices.Contains(cfg.ArtifactTypes, ref.ArtifactType) {
			out = append(out, ref)
		}
	}
	return out
}

func addName(existing, name string) string {
	if existing == "" {
		return name
	}
	m := map[string]struct{}{}
	for n := range strings.SplitSeq(existing, ",") {
		m[n] = struct{}{}
	}
	m[name] = struct{}{}
	var names []string
	for n := range m {
		names = append(names, n)
	}
	slices.Sort(names)
	return strings.Join(names, ",")
}

func readArtifactType(dt []byte) string {
	var mfst ocispecs.Manifest
	if err := json.Unmarshal(dt, &mfst); err != nil {
		return ""
	}
	return mfst.ArtifactType
}
