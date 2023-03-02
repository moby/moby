package cache

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/reference"
	"github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/bklog"
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

// GetRemotes gets []*solver.Remote from content store for this ref (potentially pulling lazily).
// Compressionopt can be used to specify the compression type of blobs. If Force is true, the compression
// type is applied to all blobs in the chain. If Force is false, it's applied only to the newly created
// layers. If all is true, all available chains that has the specified compression type of topmost blob are
// appended to the result.
// Note: Use WorkerRef.GetRemotes instead as moby integration requires custom GetRemotes implementation.
func (sr *immutableRef) GetRemotes(ctx context.Context, createIfNeeded bool, refCfg config.RefConfig, all bool, s session.Group) ([]*solver.Remote, error) {
	ctx, done, err := leaseutil.WithLease(ctx, sr.cm.LeaseManager, leaseutil.MakeTemporary)
	if err != nil {
		return nil, err
	}
	defer done(ctx)

	// fast path if compression variants aren't required
	// NOTE: compressionopt is applied only to *newly created layers* if Force != true.
	remote, err := sr.getRemote(ctx, createIfNeeded, refCfg, s)
	if err != nil {
		return nil, err
	}
	if !all || refCfg.Compression.Force || len(remote.Descriptors) == 0 {
		return []*solver.Remote{remote}, nil // early return if compression variants aren't required
	}

	// Search all available remotes that has the topmost blob with the specified
	// compression with all combination of copmressions
	res := []*solver.Remote{remote}
	topmost, parentChain := remote.Descriptors[len(remote.Descriptors)-1], remote.Descriptors[:len(remote.Descriptors)-1]
	vDesc, err := getBlobWithCompression(ctx, sr.cm.ContentStore, topmost, refCfg.Compression.Type)
	if err != nil {
		return res, nil // compression variant doesn't exist. return the main blob only.
	}

	var variants []*solver.Remote
	if len(parentChain) == 0 {
		variants = append(variants, &solver.Remote{
			Descriptors: []ocispecs.Descriptor{vDesc},
			Provider:    sr.cm.ContentStore,
		})
	} else {
		// get parents with all combination of all available compressions.
		parents, err := getAvailableBlobs(ctx, sr.cm.ContentStore, &solver.Remote{
			Descriptors: parentChain,
			Provider:    remote.Provider,
		})
		if err != nil {
			return nil, err
		}
		variants = appendRemote(parents, vDesc, sr.cm.ContentStore)
	}

	// Return the main remote and all its compression variants.
	// NOTE: Because compressionopt is applied only to *newly created layers* in the main remote (i.e. res[0]),
	//       it's possible that the main remote doesn't contain any blobs of the compressionopt.Type.
	//       The topmost blob of the variants (res[1:]) is guaranteed to be the compressionopt.Type.
	res = append(res, variants...)
	return res, nil
}

func appendRemote(parents []*solver.Remote, desc ocispecs.Descriptor, p content.Provider) (res []*solver.Remote) {
	for _, pRemote := range parents {
		provider := contentutil.NewMultiProvider(pRemote.Provider)
		provider.Add(desc.Digest, p)
		res = append(res, &solver.Remote{
			Descriptors: append(pRemote.Descriptors, desc),
			Provider:    provider,
		})
	}
	return
}

func getAvailableBlobs(ctx context.Context, cs content.Store, chain *solver.Remote) ([]*solver.Remote, error) {
	if len(chain.Descriptors) == 0 {
		return nil, nil
	}
	target, parentChain := chain.Descriptors[len(chain.Descriptors)-1], chain.Descriptors[:len(chain.Descriptors)-1]
	parents, err := getAvailableBlobs(ctx, cs, &solver.Remote{
		Descriptors: parentChain,
		Provider:    chain.Provider,
	})
	if err != nil {
		return nil, err
	}
	var descs []ocispecs.Descriptor
	if err := walkBlob(ctx, cs, target, func(desc ocispecs.Descriptor) bool {
		descs = append(descs, desc)
		return true
	}); err != nil {
		bklog.G(ctx).WithError(err).Warn("failed to walk variant blob") // is not a critical error at this moment.
	}
	var res []*solver.Remote
	for _, desc := range descs {
		desc := desc
		if len(parents) == 0 { // bottommost ref
			res = append(res, &solver.Remote{
				Descriptors: []ocispecs.Descriptor{desc},
				Provider:    cs,
			})
			continue
		}
		res = append(res, appendRemote(parents, desc, cs)...)
	}
	if len(res) == 0 {
		// no available compression blobs for this blob. return the original blob.
		if len(parents) == 0 { // bottommost ref
			return []*solver.Remote{chain}, nil
		}
		return appendRemote(parents, target, chain.Provider), nil
	}
	return res, nil
}

func (sr *immutableRef) getRemote(ctx context.Context, createIfNeeded bool, refCfg config.RefConfig, s session.Group) (*solver.Remote, error) {
	err := sr.computeBlobChain(ctx, createIfNeeded, refCfg.Compression, s)
	if err != nil {
		return nil, err
	}

	chain := sr.layerChain()
	mproviderBase := contentutil.NewMultiProvider(nil)
	mprovider := &lazyMultiProvider{mprovider: mproviderBase}
	remote := &solver.Remote{
		Provider: mprovider,
	}
	for _, ref := range chain {
		desc, err := ref.ociDesc(ctx, sr.descHandlers, refCfg.PreferNonDistributable)
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

		if needsForceCompression(ctx, sr.cm.ContentStore, desc, refCfg) {
			if needs, err := refCfg.Compression.Type.NeedsConversion(ctx, sr.cm.ContentStore, desc); err != nil {
				return nil, err
			} else if needs {
				// ensure the compression type.
				// compressed blob must be created and stored in the content store.
				blobDesc, err := getBlobWithCompressionWithRetry(ctx, ref, refCfg.Compression, s)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to get compression blob %q", refCfg.Compression.Type)
				}
				newDesc := desc
				newDesc.MediaType = blobDesc.MediaType
				newDesc.Digest = blobDesc.Digest
				newDesc.Size = blobDesc.Size
				newDesc.URLs = blobDesc.URLs
				newDesc.Annotations = nil
				if len(addAnnotations) > 0 || len(blobDesc.Annotations) > 0 {
					newDesc.Annotations = make(map[string]string)
				}
				for _, k := range addAnnotations {
					newDesc.Annotations[k] = desc.Annotations[k]
				}
				for k, v := range blobDesc.Annotations {
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

func getBlobWithCompressionWithRetry(ctx context.Context, ref *immutableRef, comp compression.Config, s session.Group) (ocispecs.Descriptor, error) {
	if blobDesc, err := ref.getBlobWithCompression(ctx, comp.Type); err == nil {
		return blobDesc, nil
	}
	if err := ensureCompression(ctx, ref, comp, s); err != nil {
		return ocispecs.Descriptor{}, errors.Wrapf(err, "failed to get and ensure compression type of %q", comp.Type)
	}
	return ref.getBlobWithCompression(ctx, comp.Type)
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
		defer func() {
			if rerr == nil {
				rerr = p.ref.linkBlob(ctx, p.desc)
			}
		}()

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

		return nil, nil
	})
	return err
}
