package cache

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/reference"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/progress/logs"
	"github.com/moby/buildkit/util/pull/pullprogress"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Unlazier interface {
	Unlazy(ctx context.Context) error
}

// GetRemote gets a *solver.Remote from content store for this ref (potentially pulling lazily).
// Note: Use WorkerRef.GetRemote instead as moby integration requires custom GetRemote implementation.
func (sr *immutableRef) GetRemote(ctx context.Context, createIfNeeded bool, compressionType compression.Type, forceCompression bool, s session.Group) (*solver.Remote, error) {
	ctx, done, err := leaseutil.WithLease(ctx, sr.cm.LeaseManager, leaseutil.MakeTemporary)
	if err != nil {
		return nil, err
	}
	defer done(ctx)

	err = sr.computeBlobChain(ctx, createIfNeeded, compressionType, forceCompression, s)
	if err != nil {
		return nil, err
	}

	chain := sr.parentRefChain()
	mproviderBase := contentutil.NewMultiProvider(nil)
	mprovider := &lazyMultiProvider{mprovider: mproviderBase}
	remote := &solver.Remote{
		Provider: mprovider,
	}
	for _, ref := range chain {
		desc, err := ref.ociDesc(ctx, sr.descHandlers)
		if err != nil {
			return nil, err
		}

		// NOTE: The media type might be missing for some migrated ones
		// from before lease based storage. If so, we should detect
		// the media type from blob data.
		//
		// Discussion: https://github.com/moby/buildkit/pull/1277#discussion_r352795429
		if desc.MediaType == "" {
			desc.MediaType, err = compression.DetectLayerMediaType(ctx, sr.cm.ContentStore, desc.Digest, false)
			if err != nil {
				return nil, err
			}
		}

		// update distribution source annotation for lazy-refs (non-lazy refs
		// will already have their dsl stored in the content store, which is
		// used by the push handlers)
		var addAnnotations []string
		isLazy, err := ref.isLazy(ctx)
		if err != nil {
			return nil, err
		} else if isLazy {
			imageRefs := ref.getImageRefs()
			for _, imageRef := range imageRefs {
				refspec, err := reference.Parse(imageRef)
				if err != nil {
					return nil, err
				}

				u, err := url.Parse("dummy://" + refspec.Locator)
				if err != nil {
					return nil, err
				}

				source, repo := u.Hostname(), strings.TrimPrefix(u.Path, "/")
				if desc.Annotations == nil {
					desc.Annotations = make(map[string]string)
				}
				dslKey := fmt.Sprintf("%s.%s", "containerd.io/distribution.source", source)

				var existingRepos []string
				if existings, ok := desc.Annotations[dslKey]; ok {
					existingRepos = strings.Split(existings, ",")
				}
				addNewRepo := true
				for _, existing := range existingRepos {
					if existing == repo {
						addNewRepo = false
						break
					}
				}
				if addNewRepo {
					existingRepos = append(existingRepos, repo)
				}
				desc.Annotations[dslKey] = strings.Join(existingRepos, ",")
				addAnnotations = append(addAnnotations, dslKey)
			}
		}

		if forceCompression {
			if needs, err := needsConversion(ctx, sr.cm.ContentStore, desc, compressionType); err != nil {
				return nil, err
			} else if needs {
				// ensure the compression type.
				// compressed blob must be created and stored in the content store.
				blobDesc, err := ref.getCompressionBlob(ctx, compressionType)
				if err != nil {
					return nil, errors.Wrapf(err, "compression blob for %q not found", compressionType)
				}
				newDesc := desc
				newDesc.MediaType = blobDesc.MediaType
				newDesc.Digest = blobDesc.Digest
				newDesc.Size = blobDesc.Size
				newDesc.Annotations = nil
				for _, k := range addAnnotations {
					newDesc.Annotations[k] = desc.Annotations[k]
				}
				for k, v := range blobDesc.Annotations {
					if newDesc.Annotations == nil {
						newDesc.Annotations = make(map[string]string)
					}
					newDesc.Annotations[k] = v
				}
				desc = newDesc
			}
		}

		remote.Descriptors = append(remote.Descriptors, desc)
		mprovider.Add(lazyRefProvider{
			ref:     ref,
			desc:    desc,
			dh:      sr.descHandlers[desc.Digest],
			session: s,
		})
	}
	return remote, nil
}

type lazyMultiProvider struct {
	mprovider *contentutil.MultiProvider
	plist     []lazyRefProvider
}

func (mp *lazyMultiProvider) Add(p lazyRefProvider) {
	mp.mprovider.Add(p.desc.Digest, p)
	mp.plist = append(mp.plist, p)
}

func (mp *lazyMultiProvider) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	return mp.mprovider.ReaderAt(ctx, desc)
}

func (mp *lazyMultiProvider) Unlazy(ctx context.Context) error {
	eg, egctx := errgroup.WithContext(ctx)
	for _, p := range mp.plist {
		p := p
		eg.Go(func() error {
			return p.Unlazy(egctx)
		})
	}
	return eg.Wait()
}

type lazyRefProvider struct {
	ref     *immutableRef
	desc    ocispecs.Descriptor
	dh      *DescHandler
	session session.Group
}

func (p lazyRefProvider) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	if desc.Digest != p.desc.Digest {
		return nil, errdefs.ErrNotFound
	}
	if err := p.Unlazy(ctx); err != nil {
		return nil, err
	}
	return p.ref.cm.ContentStore.ReaderAt(ctx, desc)
}

func (p lazyRefProvider) Unlazy(ctx context.Context) error {
	_, err := p.ref.cm.unlazyG.Do(ctx, string(p.desc.Digest), func(ctx context.Context) (_ interface{}, rerr error) {
		if isLazy, err := p.ref.isLazy(ctx); err != nil {
			return nil, err
		} else if !isLazy {
			return nil, nil
		}

		if p.dh == nil {
			// shouldn't happen, if you have a lazy immutable ref it already should be validated
			// that descriptor handlers exist for it
			return nil, errors.New("unexpected nil descriptor handler")
		}

		if p.dh.Progress != nil {
			var stopProgress func(error)
			ctx, stopProgress = p.dh.Progress.Start(ctx)
			defer stopProgress(rerr)
		}

		// For now, just pull down the whole content and then return a ReaderAt from the local content
		// store. If efficient partial reads are desired in the future, something more like a "tee"
		// that caches remote partial reads to a local store may need to replace this.
		err := contentutil.Copy(ctx, p.ref.cm.ContentStore, &pullprogress.ProviderWithProgress{
			Provider: p.dh.Provider(p.session),
			Manager:  p.ref.cm.ContentStore,
		}, p.desc, p.dh.Ref, logs.LoggerFromContext(ctx))
		if err != nil {
			return nil, err
		}

		if imageRefs := p.ref.getImageRefs(); len(imageRefs) > 0 {
			// just use the first image ref, it's arbitrary
			imageRef := imageRefs[0]
			if p.ref.GetDescription() == "" {
				if err := p.ref.SetDescription("pulled from " + imageRef); err != nil {
					return nil, err
				}
			}
		}

		compressionType := compression.FromMediaType(p.desc.MediaType)
		if compressionType == compression.UnknownCompression {
			return nil, errors.Errorf("unhandled layer media type: %q", p.desc.MediaType)
		}

		if err := p.ref.addCompressionBlob(ctx, p.desc, compressionType); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}
