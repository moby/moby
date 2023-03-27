package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/pkg/userns"
	"github.com/containerd/containerd/snapshots"
	"github.com/docker/docker/pkg/idtools"
	"github.com/hashicorp/go-multierror"
	"github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/progress"
	rootlessmountopts "github.com/moby/buildkit/util/rootless/mountopts"
	"github.com/moby/buildkit/util/winlayers"
	"github.com/moby/sys/mountinfo"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var additionalAnnotations = append(compression.EStargzAnnotations, containerdUncompressed)

// Ref is a reference to cacheable objects.
type Ref interface {
	Mountable
	RefMetadata
	Release(context.Context) error
	IdentityMapping() *idtools.IdentityMapping
	DescHandler(digest.Digest) *DescHandler
}

type ImmutableRef interface {
	Ref
	Clone() ImmutableRef
	// Finalize commits the snapshot to the driver if it's not already.
	// This means the snapshot can no longer be mounted as mutable.
	Finalize(context.Context) error

	Extract(ctx context.Context, s session.Group) error // +progress
	GetRemotes(ctx context.Context, createIfNeeded bool, cfg config.RefConfig, all bool, s session.Group) ([]*solver.Remote, error)
	LayerChain() RefList
	FileList(ctx context.Context, s session.Group) ([]string, error)
}

type MutableRef interface {
	Ref
	Commit(context.Context) (ImmutableRef, error)
}

type Mountable interface {
	Mount(ctx context.Context, readonly bool, s session.Group) (snapshot.Mountable, error)
}

type ref interface {
	shouldUpdateLastUsed() bool
}

type cacheRecord struct {
	cm *cacheManager
	mu *sync.Mutex // the mutex is shared by records sharing data

	mutable bool
	refs    map[ref]struct{}
	parentRefs
	*cacheMetadata

	// dead means record is marked as deleted
	dead bool

	mountCache snapshot.Mountable

	sizeG flightcontrol.Group

	// these are filled if multiple refs point to same data
	equalMutable   *mutableRef
	equalImmutable *immutableRef

	layerDigestChainCache []digest.Digest
}

// hold ref lock before calling
func (cr *cacheRecord) ref(triggerLastUsed bool, descHandlers DescHandlers, pg progress.Controller) *immutableRef {
	ref := &immutableRef{
		cacheRecord:     cr,
		triggerLastUsed: triggerLastUsed,
		descHandlers:    descHandlers,
		progress:        pg,
	}
	cr.refs[ref] = struct{}{}
	return ref
}

// hold ref lock before calling
func (cr *cacheRecord) mref(triggerLastUsed bool, descHandlers DescHandlers) *mutableRef {
	ref := &mutableRef{
		cacheRecord:     cr,
		triggerLastUsed: triggerLastUsed,
		descHandlers:    descHandlers,
	}
	cr.refs[ref] = struct{}{}
	return ref
}

// parentRefs is a disjoint union type that holds either a single layerParent for this record, a list
// of parents if this is a merged record or all nil fields if this record has no parents. At most one
// field should be non-nil at a time.
type parentRefs struct {
	layerParent  *immutableRef
	mergeParents []*immutableRef
	diffParents  *diffParents
}

type diffParents struct {
	lower *immutableRef
	upper *immutableRef
}

// caller must hold cacheManager.mu
func (p parentRefs) release(ctx context.Context) (rerr error) {
	switch {
	case p.layerParent != nil:
		p.layerParent.mu.Lock()
		defer p.layerParent.mu.Unlock()
		rerr = p.layerParent.release(ctx)
	case len(p.mergeParents) > 0:
		for i, parent := range p.mergeParents {
			if parent == nil {
				continue
			}
			parent.mu.Lock()
			if err := parent.release(ctx); err != nil {
				rerr = multierror.Append(rerr, err).ErrorOrNil()
			} else {
				p.mergeParents[i] = nil
			}
			parent.mu.Unlock()
		}
	case p.diffParents != nil:
		if p.diffParents.lower != nil {
			p.diffParents.lower.mu.Lock()
			defer p.diffParents.lower.mu.Unlock()
			if err := p.diffParents.lower.release(ctx); err != nil {
				rerr = multierror.Append(rerr, err).ErrorOrNil()
			} else {
				p.diffParents.lower = nil
			}
		}
		if p.diffParents.upper != nil {
			p.diffParents.upper.mu.Lock()
			defer p.diffParents.upper.mu.Unlock()
			if err := p.diffParents.upper.release(ctx); err != nil {
				rerr = multierror.Append(rerr, err).ErrorOrNil()
			} else {
				p.diffParents.upper = nil
			}
		}
	}

	return rerr
}

func (p parentRefs) clone() parentRefs {
	switch {
	case p.layerParent != nil:
		p.layerParent = p.layerParent.clone()
	case len(p.mergeParents) > 0:
		newParents := make([]*immutableRef, len(p.mergeParents))
		for i, p := range p.mergeParents {
			newParents[i] = p.clone()
		}
		p.mergeParents = newParents
	case p.diffParents != nil:
		newDiffParents := &diffParents{}
		if p.diffParents.lower != nil {
			newDiffParents.lower = p.diffParents.lower.clone()
		}
		if p.diffParents.upper != nil {
			newDiffParents.upper = p.diffParents.upper.clone()
		}
		p.diffParents = newDiffParents
	}
	return p
}

type refKind int

const (
	BaseLayer refKind = iota
	Layer
	Merge
	Diff
)

func (cr *cacheRecord) kind() refKind {
	if len(cr.mergeParents) > 0 {
		return Merge
	}
	if cr.diffParents != nil {
		return Diff
	}
	if cr.layerParent != nil {
		return Layer
	}
	return BaseLayer
}

// hold ref lock before calling
func (cr *cacheRecord) isDead() bool {
	return cr.dead || (cr.equalImmutable != nil && cr.equalImmutable.dead) || (cr.equalMutable != nil && cr.equalMutable.dead)
}

var errSkipWalk = errors.New("skip")

// walkAncestors calls the provided func on cr and each of its ancestors, counting layer,
// diff, and merge parents. It starts at cr and does a depth-first walk to parents. It will visit
// a record and its parents multiple times if encountered more than once. It will only skip
// visiting parents of a record if errSkipWalk is returned. If any other error is returned,
// the walk will stop and return the error to the caller.
func (cr *cacheRecord) walkAncestors(f func(*cacheRecord) error) error {
	curs := []*cacheRecord{cr}
	for len(curs) > 0 {
		cur := curs[len(curs)-1]
		curs = curs[:len(curs)-1]
		if err := f(cur); err != nil {
			if errors.Is(err, errSkipWalk) {
				continue
			}
			return err
		}
		switch cur.kind() {
		case Layer:
			curs = append(curs, cur.layerParent.cacheRecord)
		case Merge:
			for _, p := range cur.mergeParents {
				curs = append(curs, p.cacheRecord)
			}
		case Diff:
			if cur.diffParents.lower != nil {
				curs = append(curs, cur.diffParents.lower.cacheRecord)
			}
			if cur.diffParents.upper != nil {
				curs = append(curs, cur.diffParents.upper.cacheRecord)
			}
		}
	}
	return nil
}

// walkUniqueAncestors calls walkAncestors but skips a record if it's already been visited.
func (cr *cacheRecord) walkUniqueAncestors(f func(*cacheRecord) error) error {
	memo := make(map[*cacheRecord]struct{})
	return cr.walkAncestors(func(cr *cacheRecord) error {
		if _, ok := memo[cr]; ok {
			return errSkipWalk
		}
		memo[cr] = struct{}{}
		return f(cr)
	})
}

func (cr *cacheRecord) isLazy(ctx context.Context) (bool, error) {
	if !cr.getBlobOnly() {
		return false, nil
	}
	dgst := cr.getBlob()
	// special case for moby where there is no compressed blob (empty digest)
	if dgst == "" {
		return false, nil
	}
	_, err := cr.cm.ContentStore.Info(ctx, dgst)
	if errors.Is(err, errdefs.ErrNotFound) {
		return true, nil
	} else if err != nil {
		return false, err
	}

	// If the snapshot is a remote snapshot, this layer is lazy.
	if info, err := cr.cm.Snapshotter.Stat(ctx, cr.getSnapshotID()); err == nil {
		if _, ok := info.Labels["containerd.io/snapshot/remote"]; ok {
			return true, nil
		}
	}

	return false, nil
}

func (cr *cacheRecord) IdentityMapping() *idtools.IdentityMapping {
	return cr.cm.IdentityMapping()
}

func (cr *cacheRecord) viewLeaseID() string {
	return cr.ID() + "-view"
}

func (cr *cacheRecord) compressionVariantsLeaseID() string {
	return cr.ID() + "-variants"
}

func (cr *cacheRecord) viewSnapshotID() string {
	return cr.getSnapshotID() + "-view"
}

func (cr *cacheRecord) size(ctx context.Context) (int64, error) {
	// this expects that usage() is implemented lazily
	s, err := cr.sizeG.Do(ctx, cr.ID(), func(ctx context.Context) (interface{}, error) {
		cr.mu.Lock()
		s := cr.getSize()
		if s != sizeUnknown {
			cr.mu.Unlock()
			return s, nil
		}
		driverID := cr.getSnapshotID()
		if cr.equalMutable != nil {
			driverID = cr.equalMutable.getSnapshotID()
		}
		cr.mu.Unlock()
		var usage snapshots.Usage
		if !cr.getBlobOnly() {
			var err error
			usage, err = cr.cm.Snapshotter.Usage(ctx, driverID)
			if err != nil {
				cr.mu.Lock()
				isDead := cr.isDead()
				cr.mu.Unlock()
				if isDead {
					return int64(0), nil
				}
				if !errors.Is(err, errdefs.ErrNotFound) {
					return s, errors.Wrapf(err, "failed to get usage for %s", cr.ID())
				}
			}
		}
		if dgst := cr.getBlob(); dgst != "" {
			added := make(map[digest.Digest]struct{})
			info, err := cr.cm.ContentStore.Info(ctx, digest.Digest(dgst))
			if err == nil {
				usage.Size += info.Size
				added[digest.Digest(dgst)] = struct{}{}
			}
			walkBlobVariantsOnly(ctx, cr.cm.ContentStore, digest.Digest(dgst), func(desc ocispecs.Descriptor) bool {
				if _, ok := added[desc.Digest]; !ok {
					if info, err := cr.cm.ContentStore.Info(ctx, desc.Digest); err == nil {
						usage.Size += info.Size
						added[desc.Digest] = struct{}{}
					}
				}
				return true
			}, nil)
		}
		cr.mu.Lock()
		cr.queueSize(usage.Size)
		if err := cr.commitMetadata(); err != nil {
			cr.mu.Unlock()
			return s, err
		}
		cr.mu.Unlock()
		return usage.Size, nil
	})
	if err != nil {
		return 0, err
	}
	return s.(int64), nil
}

// caller must hold cr.mu
func (cr *cacheRecord) mount(ctx context.Context, s session.Group) (_ snapshot.Mountable, rerr error) {
	if cr.mountCache != nil {
		return cr.mountCache, nil
	}

	var mountSnapshotID string
	if cr.mutable {
		mountSnapshotID = cr.getSnapshotID()
	} else if cr.equalMutable != nil {
		mountSnapshotID = cr.equalMutable.getSnapshotID()
	} else {
		mountSnapshotID = cr.viewSnapshotID()
		if _, err := cr.cm.LeaseManager.Create(ctx, func(l *leases.Lease) error {
			l.ID = cr.viewLeaseID()
			l.Labels = map[string]string{
				"containerd.io/gc.flat": time.Now().UTC().Format(time.RFC3339Nano),
			}
			return nil
		}, leaseutil.MakeTemporary); err != nil && !errdefs.IsAlreadyExists(err) {
			return nil, err
		}
		defer func() {
			if rerr != nil {
				cr.cm.LeaseManager.Delete(context.TODO(), leases.Lease{ID: cr.viewLeaseID()})
			}
		}()
		if err := cr.cm.LeaseManager.AddResource(ctx, leases.Lease{ID: cr.viewLeaseID()}, leases.Resource{
			ID:   mountSnapshotID,
			Type: "snapshots/" + cr.cm.Snapshotter.Name(),
		}); err != nil && !errdefs.IsAlreadyExists(err) {
			return nil, err
		}
		// Return the mount direct from View rather than setting it using the Mounts call below.
		// The two are equivalent for containerd snapshotters but the moby snapshotter requires
		// the use of the mountable returned by View in this case.
		mnts, err := cr.cm.Snapshotter.View(ctx, mountSnapshotID, cr.getSnapshotID())
		if err != nil && !errdefs.IsAlreadyExists(err) {
			return nil, err
		}
		cr.mountCache = mnts
	}

	if cr.mountCache != nil {
		return cr.mountCache, nil
	}

	mnts, err := cr.cm.Snapshotter.Mounts(ctx, mountSnapshotID)
	if err != nil {
		return nil, err
	}
	cr.mountCache = mnts
	return cr.mountCache, nil
}

// call when holding the manager lock
func (cr *cacheRecord) remove(ctx context.Context, removeSnapshot bool) error {
	delete(cr.cm.records, cr.ID())
	if removeSnapshot {
		if err := cr.cm.LeaseManager.Delete(ctx, leases.Lease{
			ID: cr.ID(),
		}); err != nil && !errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "failed to delete lease for %s", cr.ID())
		}
		if err := cr.cm.LeaseManager.Delete(ctx, leases.Lease{
			ID: cr.compressionVariantsLeaseID(),
		}); err != nil && !errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "failed to delete compression variant lease for %s", cr.ID())
		}
	}
	if err := cr.cm.MetadataStore.Clear(cr.ID()); err != nil {
		return errors.Wrapf(err, "failed to delete metadata of %s", cr.ID())
	}
	if err := cr.parentRefs.release(ctx); err != nil {
		return errors.Wrapf(err, "failed to release parents of %s", cr.ID())
	}
	return nil
}

type immutableRef struct {
	*cacheRecord
	triggerLastUsed bool
	descHandlers    DescHandlers
	// TODO:(sipsma) de-dupe progress with the same field inside descHandlers?
	progress progress.Controller
}

// Order is from parent->child, sr will be at end of slice. Refs should not
// be released as they are used internally in the underlying cacheRecords.
func (sr *immutableRef) layerChain() []*immutableRef {
	var count int
	sr.layerWalk(func(*immutableRef) {
		count++
	})
	layers := make([]*immutableRef, count)
	var index int
	sr.layerWalk(func(sr *immutableRef) {
		layers[index] = sr
		index++
	})
	return layers
}

// returns the set of cache record IDs for each layer in sr's layer chain
func (sr *immutableRef) layerSet() map[string]struct{} {
	var count int
	sr.layerWalk(func(*immutableRef) {
		count++
	})
	set := make(map[string]struct{}, count)
	sr.layerWalk(func(sr *immutableRef) {
		set[sr.ID()] = struct{}{}
	})
	return set
}

// layerWalk visits each ref representing an actual layer in the chain for
// sr (including sr). The layers are visited from lowest->highest as ordered
// in the remote for the ref.
func (sr *immutableRef) layerWalk(f func(*immutableRef)) {
	switch sr.kind() {
	case Merge:
		for _, parent := range sr.mergeParents {
			parent.layerWalk(f)
		}
	case Diff:
		lower := sr.diffParents.lower
		upper := sr.diffParents.upper
		// If upper is only one blob different from lower, then re-use that blob
		switch {
		case upper != nil && lower == nil && upper.kind() == BaseLayer:
			// upper is a single layer being diffed with scratch
			f(upper)
		case upper != nil && lower != nil && upper.kind() == Layer && upper.layerParent.ID() == lower.ID():
			// upper is a single layer on top of lower
			f(upper)
		default:
			// otherwise, the diff will be computed and turned into its own single blob
			f(sr)
		}
	case Layer:
		sr.layerParent.layerWalk(f)
		fallthrough
	case BaseLayer:
		f(sr)
	}
}

// hold cacheRecord.mu lock before calling
func (cr *cacheRecord) layerDigestChain() []digest.Digest {
	if cr.layerDigestChainCache != nil {
		return cr.layerDigestChainCache
	}
	switch cr.kind() {
	case Diff:
		if cr.getBlob() == "" && cr.diffParents.upper != nil {
			// this diff just reuses the upper blob
			cr.layerDigestChainCache = cr.diffParents.upper.layerDigestChain()
		} else {
			cr.layerDigestChainCache = append(cr.layerDigestChainCache, cr.getBlob())
		}
	case Merge:
		for _, parent := range cr.mergeParents {
			cr.layerDigestChainCache = append(cr.layerDigestChainCache, parent.layerDigestChain()...)
		}
	case Layer:
		cr.layerDigestChainCache = append(cr.layerDigestChainCache, cr.layerParent.layerDigestChain()...)
		fallthrough
	case BaseLayer:
		cr.layerDigestChainCache = append(cr.layerDigestChainCache, cr.getBlob())
	}
	return cr.layerDigestChainCache
}

type RefList []ImmutableRef

func (l RefList) Release(ctx context.Context) (rerr error) {
	for i, r := range l {
		if r == nil {
			continue
		}
		if err := r.Release(ctx); err != nil {
			rerr = multierror.Append(rerr, err).ErrorOrNil()
		} else {
			l[i] = nil
		}
	}
	return rerr
}

func (sr *immutableRef) LayerChain() RefList {
	chain := sr.layerChain()
	l := RefList(make([]ImmutableRef, len(chain)))
	for i, p := range chain {
		l[i] = p.Clone()
	}
	return l
}

func (sr *immutableRef) DescHandler(dgst digest.Digest) *DescHandler {
	return sr.descHandlers[dgst]
}

type mutableRef struct {
	*cacheRecord
	triggerLastUsed bool
	descHandlers    DescHandlers
}

func (sr *mutableRef) DescHandler(dgst digest.Digest) *DescHandler {
	return sr.descHandlers[dgst]
}

func (sr *immutableRef) clone() *immutableRef {
	sr.mu.Lock()
	ref := sr.ref(false, sr.descHandlers, sr.progress)
	sr.mu.Unlock()
	return ref
}

func (sr *immutableRef) Clone() ImmutableRef {
	return sr.clone()
}

// layertoDistributable changes the passed in media type to the "distributable" version of the media type.
func layerToDistributable(mt string) string {
	if !images.IsNonDistributable(mt) {
		// Layer is already a distributable media type (or this is not even a layer).
		// No conversion needed
		return mt
	}

	switch mt {
	case ocispecs.MediaTypeImageLayerNonDistributable:
		return ocispecs.MediaTypeImageLayer
	case ocispecs.MediaTypeImageLayerNonDistributableGzip:
		return ocispecs.MediaTypeImageLayerGzip
	case ocispecs.MediaTypeImageLayerNonDistributableZstd:
		return ocispecs.MediaTypeImageLayerZstd
	case images.MediaTypeDockerSchema2LayerForeign:
		return images.MediaTypeDockerSchema2Layer
	case images.MediaTypeDockerSchema2LayerForeignGzip:
		return images.MediaTypeDockerSchema2LayerGzip
	default:
		return mt
	}
}

func layerToNonDistributable(mt string) string {
	switch mt {
	case ocispecs.MediaTypeImageLayer:
		return ocispecs.MediaTypeImageLayerNonDistributable
	case ocispecs.MediaTypeImageLayerGzip:
		return ocispecs.MediaTypeImageLayerNonDistributableGzip
	case ocispecs.MediaTypeImageLayerZstd:
		return ocispecs.MediaTypeImageLayerNonDistributableZstd
	case images.MediaTypeDockerSchema2Layer:
		return images.MediaTypeDockerSchema2LayerForeign
	case images.MediaTypeDockerSchema2LayerForeignGzip:
		return images.MediaTypeDockerSchema2LayerForeignGzip
	default:
		return mt
	}
}

func (sr *immutableRef) ociDesc(ctx context.Context, dhs DescHandlers, preferNonDist bool) (ocispecs.Descriptor, error) {
	dgst := sr.getBlob()
	if dgst == "" {
		return ocispecs.Descriptor{}, errors.Errorf("no blob set for cache record %s", sr.ID())
	}

	desc := ocispecs.Descriptor{
		Digest:      sr.getBlob(),
		Size:        sr.getBlobSize(),
		Annotations: make(map[string]string),
		MediaType:   sr.getMediaType(),
	}

	if preferNonDist {
		if urls := sr.getURLs(); len(urls) > 0 {
			// Make sure the media type is the non-distributable version
			// We don't want to rely on the stored media type here because it could have been stored as distributable originally.
			desc.MediaType = layerToNonDistributable(desc.MediaType)
			desc.URLs = urls
		}
	}
	if len(desc.URLs) == 0 {
		// If there are no URL's, there is no reason to have this be non-dsitributable
		desc.MediaType = layerToDistributable(desc.MediaType)
	}

	if blobDesc, err := getBlobDesc(ctx, sr.cm.ContentStore, desc.Digest); err == nil {
		if blobDesc.Annotations != nil {
			desc.Annotations = blobDesc.Annotations
		}
	} else if dh, ok := dhs[desc.Digest]; ok {
		// No blob metadtata is stored in the content store. Try to get annotations from desc handlers.
		for k, v := range filterAnnotationsForSave(dh.Annotations) {
			desc.Annotations[k] = v
		}
	}

	diffID := sr.getDiffID()
	if diffID != "" {
		desc.Annotations["containerd.io/uncompressed"] = string(diffID)
	}

	createdAt := sr.GetCreatedAt()
	if !createdAt.IsZero() {
		createdAt, err := createdAt.MarshalText()
		if err != nil {
			return ocispecs.Descriptor{}, err
		}
		desc.Annotations["buildkit/createdat"] = string(createdAt)
	}

	return desc, nil
}

const (
	blobVariantGCLabel         = "containerd.io/gc.ref.content.blob-"
	blobAnnotationsLabelPrefix = "buildkit.io/blob/annotation."
	blobMediaTypeLabel         = "buildkit.io/blob/mediatype"
)

// linkBlob makes a link between this ref and the passed blob. The linked blob can be
// acquired during walkBlob. This is useful to associate a compression variant blob to
// this ref. This doesn't record the blob to the cache record (i.e. the passed blob can't
// be acquired through getBlob). Use setBlob for that purpose.
func (sr *immutableRef) linkBlob(ctx context.Context, desc ocispecs.Descriptor) error {
	if _, err := sr.cm.LeaseManager.Create(ctx, func(l *leases.Lease) error {
		l.ID = sr.compressionVariantsLeaseID()
		// do not make it flat lease to allow linking blobs using gc label
		return nil
	}); err != nil && !errdefs.IsAlreadyExists(err) {
		return err
	}
	if err := sr.cm.LeaseManager.AddResource(ctx, leases.Lease{ID: sr.compressionVariantsLeaseID()}, leases.Resource{
		ID:   desc.Digest.String(),
		Type: "content",
	}); err != nil {
		return err
	}
	cs := sr.cm.ContentStore
	blobDigest := sr.getBlob()
	info, err := cs.Info(ctx, blobDigest)
	if err != nil {
		return err
	}
	vInfo, err := cs.Info(ctx, desc.Digest)
	if err != nil {
		return err
	}
	vInfo.Labels = map[string]string{
		blobVariantGCLabel + blobDigest.String(): blobDigest.String(),
	}
	vInfo = addBlobDescToInfo(desc, vInfo)
	if _, err := cs.Update(ctx, vInfo, fieldsFromLabels(vInfo.Labels)...); err != nil {
		return err
	}
	// let the future call to size() recalcultate the new size
	sr.mu.Lock()
	sr.queueSize(sizeUnknown)
	if err := sr.commitMetadata(); err != nil {
		sr.mu.Unlock()
		return err
	}
	sr.mu.Unlock()
	if desc.Digest == blobDigest {
		return nil
	}
	info.Labels = map[string]string{
		blobVariantGCLabel + desc.Digest.String(): desc.Digest.String(),
	}
	_, err = cs.Update(ctx, info, fieldsFromLabels(info.Labels)...)
	return err
}

func (sr *immutableRef) getBlobWithCompression(ctx context.Context, compressionType compression.Type) (ocispecs.Descriptor, error) {
	if _, err := sr.cm.ContentStore.Info(ctx, sr.getBlob()); err != nil {
		return ocispecs.Descriptor{}, err
	}
	desc, err := sr.ociDesc(ctx, nil, true)
	if err != nil {
		return ocispecs.Descriptor{}, err
	}
	return getBlobWithCompression(ctx, sr.cm.ContentStore, desc, compressionType)
}

func getBlobWithCompression(ctx context.Context, cs content.Store, desc ocispecs.Descriptor, compressionType compression.Type) (ocispecs.Descriptor, error) {
	var target *ocispecs.Descriptor
	if err := walkBlob(ctx, cs, desc, func(desc ocispecs.Descriptor) bool {
		if needs, err := compressionType.NeedsConversion(ctx, cs, desc); err == nil && !needs {
			target = &desc
			return false
		}
		return true
	}); err != nil || target == nil {
		return ocispecs.Descriptor{}, errdefs.ErrNotFound
	}
	return *target, nil
}

func walkBlob(ctx context.Context, cs content.Store, desc ocispecs.Descriptor, f func(ocispecs.Descriptor) bool) error {
	if !f(desc) {
		return nil
	}
	if _, err := walkBlobVariantsOnly(ctx, cs, desc.Digest, func(desc ocispecs.Descriptor) bool { return f(desc) }, nil); err != nil {
		return err
	}
	return nil
}

func walkBlobVariantsOnly(ctx context.Context, cs content.Store, dgst digest.Digest, f func(ocispecs.Descriptor) bool, visited map[digest.Digest]struct{}) (bool, error) {
	if visited == nil {
		visited = make(map[digest.Digest]struct{})
	}
	visited[dgst] = struct{}{}
	info, err := cs.Info(ctx, dgst)
	if errors.Is(err, errdefs.ErrNotFound) {
		return true, nil
	} else if err != nil {
		return false, err
	}
	var children []digest.Digest
	for k, dgstS := range info.Labels {
		if !strings.HasPrefix(k, blobVariantGCLabel) {
			continue
		}
		cDgst, err := digest.Parse(dgstS)
		if err != nil || cDgst == dgst {
			continue
		}
		if cDesc, err := getBlobDesc(ctx, cs, cDgst); err == nil {
			if !f(cDesc) {
				return false, nil
			}
		}
		children = append(children, cDgst)
	}
	for _, c := range children {
		if _, isVisited := visited[c]; isVisited {
			continue
		}
		if isContinue, err := walkBlobVariantsOnly(ctx, cs, c, f, visited); !isContinue || err != nil {
			return isContinue, err
		}
	}
	return true, nil
}

func getBlobDesc(ctx context.Context, cs content.Store, dgst digest.Digest) (ocispecs.Descriptor, error) {
	info, err := cs.Info(ctx, dgst)
	if err != nil {
		return ocispecs.Descriptor{}, err
	}
	if info.Labels == nil {
		return ocispecs.Descriptor{}, errors.Errorf("no blob metadata is stored for %q", info.Digest)
	}
	mt, ok := info.Labels[blobMediaTypeLabel]
	if !ok {
		return ocispecs.Descriptor{}, errors.Errorf("no media type is stored for %q", info.Digest)
	}
	desc := ocispecs.Descriptor{
		Digest:    info.Digest,
		Size:      info.Size,
		MediaType: mt,
	}
	for k, v := range info.Labels {
		if strings.HasPrefix(k, blobAnnotationsLabelPrefix) {
			if desc.Annotations == nil {
				desc.Annotations = make(map[string]string)
			}
			desc.Annotations[strings.TrimPrefix(k, blobAnnotationsLabelPrefix)] = v
		}
	}
	if len(desc.URLs) == 0 {
		// If there are no URL's, there is no reason to have this be non-dsitributable
		desc.MediaType = layerToDistributable(desc.MediaType)
	}
	return desc, nil
}

func addBlobDescToInfo(desc ocispecs.Descriptor, info content.Info) content.Info {
	if _, ok := info.Labels[blobMediaTypeLabel]; ok {
		return info // descriptor information already stored
	}
	if info.Labels == nil {
		info.Labels = make(map[string]string)
	}
	info.Labels[blobMediaTypeLabel] = desc.MediaType
	for k, v := range filterAnnotationsForSave(desc.Annotations) {
		info.Labels[blobAnnotationsLabelPrefix+k] = v
	}
	return info
}

func filterAnnotationsForSave(a map[string]string) (b map[string]string) {
	if a == nil {
		return nil
	}
	for _, k := range additionalAnnotations {
		v, ok := a[k]
		if !ok {
			continue
		}
		if b == nil {
			b = make(map[string]string)
		}
		b[k] = v
	}
	return
}

func fieldsFromLabels(labels map[string]string) (fields []string) {
	for k := range labels {
		fields = append(fields, "labels."+k)
	}
	return
}

func (sr *immutableRef) Mount(ctx context.Context, readonly bool, s session.Group) (_ snapshot.Mountable, rerr error) {
	if sr.equalMutable != nil && !readonly {
		if err := sr.Finalize(ctx); err != nil {
			return nil, err
		}
	}

	if err := sr.Extract(ctx, s); err != nil {
		return nil, err
	}

	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.mountCache != nil {
		if readonly {
			return setReadonly(sr.mountCache), nil
		}
		return sr.mountCache, nil
	}

	var mnt snapshot.Mountable
	if sr.cm.Snapshotter.Name() == "stargz" {
		if err := sr.withRemoteSnapshotLabelsStargzMode(ctx, s, func() {
			mnt, rerr = sr.mount(ctx, s)
		}); err != nil {
			return nil, err
		}
	} else {
		mnt, rerr = sr.mount(ctx, s)
	}
	if rerr != nil {
		return nil, rerr
	}

	if readonly {
		mnt = setReadonly(mnt)
	}
	return mnt, nil
}

func (sr *immutableRef) Extract(ctx context.Context, s session.Group) (rerr error) {
	if (sr.kind() == Layer || sr.kind() == BaseLayer) && !sr.getBlobOnly() {
		return nil
	}

	if sr.cm.Snapshotter.Name() == "stargz" {
		if err := sr.withRemoteSnapshotLabelsStargzMode(ctx, s, func() {
			if rerr = sr.prepareRemoteSnapshotsStargzMode(ctx, s); rerr != nil {
				return
			}
			rerr = sr.unlazy(ctx, sr.descHandlers, sr.progress, s, true)
		}); err != nil {
			return err
		}
		return rerr
	}

	return sr.unlazy(ctx, sr.descHandlers, sr.progress, s, true)
}

func (sr *immutableRef) withRemoteSnapshotLabelsStargzMode(ctx context.Context, s session.Group, f func()) error {
	dhs := sr.descHandlers
	for _, r := range sr.layerChain() {
		r := r
		info, err := r.cm.Snapshotter.Stat(ctx, r.getSnapshotID())
		if err != nil && !errdefs.IsNotFound(err) {
			return err
		} else if errdefs.IsNotFound(err) {
			continue // This snpashot doesn't exist; skip
		} else if _, ok := info.Labels["containerd.io/snapshot/remote"]; !ok {
			continue // This isn't a remote snapshot; skip
		}
		dh := dhs[digest.Digest(r.getBlob())]
		if dh == nil {
			continue // no info passed; skip
		}

		// Append temporary labels (based on dh.SnapshotLabels) as hints for remote snapshots.
		// For avoiding collosion among calls, keys of these tmp labels contain an unique ID.
		flds, labels := makeTmpLabelsStargzMode(snapshots.FilterInheritedLabels(dh.SnapshotLabels), s)
		info.Labels = labels
		if _, err := r.cm.Snapshotter.Update(ctx, info, flds...); err != nil {
			return errors.Wrapf(err, "failed to add tmp remote labels for remote snapshot")
		}
		defer func() {
			for k := range info.Labels {
				info.Labels[k] = "" // Remove labels appended in this call
			}
			if _, err := r.cm.Snapshotter.Update(ctx, info, flds...); err != nil {
				logrus.Warn(errors.Wrapf(err, "failed to remove tmp remote labels"))
			}
		}()

		continue
	}

	f()

	return nil
}

func (sr *immutableRef) prepareRemoteSnapshotsStargzMode(ctx context.Context, s session.Group) error {
	_, err := sr.sizeG.Do(ctx, sr.ID()+"-prepare-remote-snapshot", func(ctx context.Context) (_ interface{}, rerr error) {
		dhs := sr.descHandlers
		for _, r := range sr.layerChain() {
			r := r
			snapshotID := r.getSnapshotID()
			if _, err := r.cm.Snapshotter.Stat(ctx, snapshotID); err == nil {
				continue
			}

			dh := dhs[digest.Digest(r.getBlob())]
			if dh == nil {
				// We cannot prepare remote snapshots without descHandler.
				return nil, nil
			}

			// tmpLabels contains dh.SnapshotLabels + session IDs. All keys contain
			// an unique ID for avoiding the collision among snapshotter API calls to
			// this snapshot. tmpLabels will be removed at the end of this function.
			defaultLabels := snapshots.FilterInheritedLabels(dh.SnapshotLabels)
			if defaultLabels == nil {
				defaultLabels = make(map[string]string)
			}
			tmpFields, tmpLabels := makeTmpLabelsStargzMode(defaultLabels, s)
			defaultLabels["containerd.io/snapshot.ref"] = snapshotID

			// Prepare remote snapshots
			var (
				key  = fmt.Sprintf("tmp-%s %s", identity.NewID(), r.getChainID())
				opts = []snapshots.Opt{
					snapshots.WithLabels(defaultLabels),
					snapshots.WithLabels(tmpLabels),
				}
			)
			parentID := ""
			if r.layerParent != nil {
				parentID = r.layerParent.getSnapshotID()
			}
			if err := r.cm.Snapshotter.Prepare(ctx, key, parentID, opts...); err != nil {
				if errdefs.IsAlreadyExists(err) {
					// Check if the targeting snapshot ID has been prepared as
					// a remote snapshot in the snapshotter.
					info, err := r.cm.Snapshotter.Stat(ctx, snapshotID)
					if err == nil { // usable as remote snapshot without unlazying.
						defer func() {
							// Remove tmp labels appended in this func
							for k := range tmpLabels {
								info.Labels[k] = ""
							}
							if _, err := r.cm.Snapshotter.Update(ctx, info, tmpFields...); err != nil {
								logrus.Warn(errors.Wrapf(err,
									"failed to remove tmp remote labels after prepare"))
							}
						}()

						// Try the next layer as well.
						continue
					}
				}
			}

			// This layer and all upper layers cannot be prepared without unlazying.
			break
		}

		return nil, nil
	})
	return err
}

func makeTmpLabelsStargzMode(labels map[string]string, s session.Group) (fields []string, res map[string]string) {
	res = make(map[string]string)
	// Append unique ID to labels for avoiding collision of labels among calls
	id := identity.NewID()
	for k, v := range labels {
		tmpKey := k + "." + id
		fields = append(fields, "labels."+tmpKey)
		res[tmpKey] = v
	}
	for i, sid := range session.AllSessionIDs(s) {
		sidKey := "containerd.io/snapshot/remote/stargz.session." + fmt.Sprintf("%d", i) + "." + id
		fields = append(fields, "labels."+sidKey)
		res[sidKey] = sid
	}
	return
}

func (sr *immutableRef) unlazy(ctx context.Context, dhs DescHandlers, pg progress.Controller, s session.Group, topLevel bool) error {
	_, err := sr.sizeG.Do(ctx, sr.ID()+"-unlazy", func(ctx context.Context) (_ interface{}, rerr error) {
		if _, err := sr.cm.Snapshotter.Stat(ctx, sr.getSnapshotID()); err == nil {
			return nil, nil
		}

		switch sr.kind() {
		case Merge, Diff:
			return nil, sr.unlazyDiffMerge(ctx, dhs, pg, s, topLevel)
		case Layer, BaseLayer:
			return nil, sr.unlazyLayer(ctx, dhs, pg, s)
		}
		return nil, nil
	})
	return err
}

// should be called within sizeG.Do call for this ref's ID
func (sr *immutableRef) unlazyDiffMerge(ctx context.Context, dhs DescHandlers, pg progress.Controller, s session.Group, topLevel bool) (rerr error) {
	eg, egctx := errgroup.WithContext(ctx)
	var diffs []snapshot.Diff
	sr.layerWalk(func(sr *immutableRef) {
		var diff snapshot.Diff
		switch sr.kind() {
		case Diff:
			if sr.diffParents.lower != nil {
				diff.Lower = sr.diffParents.lower.getSnapshotID()
				eg.Go(func() error {
					return sr.diffParents.lower.unlazy(egctx, dhs, pg, s, false)
				})
			}
			if sr.diffParents.upper != nil {
				diff.Upper = sr.diffParents.upper.getSnapshotID()
				eg.Go(func() error {
					return sr.diffParents.upper.unlazy(egctx, dhs, pg, s, false)
				})
			}
		case Layer:
			diff.Lower = sr.layerParent.getSnapshotID()
			fallthrough
		case BaseLayer:
			diff.Upper = sr.getSnapshotID()
			eg.Go(func() error {
				return sr.unlazy(egctx, dhs, pg, s, false)
			})
		}
		diffs = append(diffs, diff)
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	if pg != nil {
		action := "merging"
		if sr.kind() == Diff {
			action = "diffing"
		}
		progressID := sr.GetDescription()
		if topLevel {
			progressID = action
		}
		if progressID == "" {
			progressID = fmt.Sprintf("%s %s", action, sr.ID())
		}
		_, stopProgress := pg.Start(ctx)
		defer stopProgress(rerr)
		statusDone := pg.Status(progressID, action)
		defer statusDone()
	}

	return sr.cm.Snapshotter.Merge(ctx, sr.getSnapshotID(), diffs)
}

// should be called within sizeG.Do call for this ref's ID
func (sr *immutableRef) unlazyLayer(ctx context.Context, dhs DescHandlers, pg progress.Controller, s session.Group) (rerr error) {
	if !sr.getBlobOnly() {
		return nil
	}

	if sr.cm.Applier == nil {
		return errors.New("unlazy requires an applier")
	}

	if _, ok := leases.FromContext(ctx); !ok {
		leaseCtx, done, err := leaseutil.WithLease(ctx, sr.cm.LeaseManager, leaseutil.MakeTemporary)
		if err != nil {
			return err
		}
		defer done(leaseCtx)
		ctx = leaseCtx
	}

	if sr.GetLayerType() == "windows" {
		ctx = winlayers.UseWindowsLayerMode(ctx)
	}

	eg, egctx := errgroup.WithContext(ctx)

	parentID := ""
	if sr.layerParent != nil {
		eg.Go(func() error {
			if err := sr.layerParent.unlazy(egctx, dhs, pg, s, false); err != nil {
				return err
			}
			parentID = sr.layerParent.getSnapshotID()
			return nil
		})
	}

	desc, err := sr.ociDesc(ctx, dhs, true)
	if err != nil {
		return err
	}
	dh := dhs[desc.Digest]

	eg.Go(func() error {
		// unlazies if needed, otherwise a no-op
		return lazyRefProvider{
			ref:     sr,
			desc:    desc,
			dh:      dh,
			session: s,
		}.Unlazy(egctx)
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	if pg == nil && dh != nil {
		pg = dh.Progress
	}
	if pg != nil {
		_, stopProgress := pg.Start(ctx)
		defer stopProgress(rerr)
		statusDone := pg.Status("extracting "+desc.Digest.String(), "extracting")
		defer statusDone()
	}

	key := fmt.Sprintf("extract-%s %s", identity.NewID(), sr.getChainID())

	err = sr.cm.Snapshotter.Prepare(ctx, key, parentID)
	if err != nil {
		return err
	}

	mountable, err := sr.cm.Snapshotter.Mounts(ctx, key)
	if err != nil {
		return err
	}
	mounts, unmount, err := mountable.Mount()
	if err != nil {
		return err
	}
	_, err = sr.cm.Applier.Apply(ctx, desc, mounts)
	if err != nil {
		unmount()
		return err
	}

	if err := unmount(); err != nil {
		return err
	}
	if err := sr.cm.Snapshotter.Commit(ctx, sr.getSnapshotID(), key); err != nil {
		if !errors.Is(err, errdefs.ErrAlreadyExists) {
			return err
		}
	}
	sr.queueBlobOnly(false)
	sr.queueSize(sizeUnknown)
	if err := sr.commitMetadata(); err != nil {
		return err
	}
	return nil
}

func (sr *immutableRef) Release(ctx context.Context) error {
	sr.cm.mu.Lock()
	defer sr.cm.mu.Unlock()

	sr.mu.Lock()
	defer sr.mu.Unlock()

	return sr.release(ctx)
}

func (sr *immutableRef) shouldUpdateLastUsed() bool {
	return sr.triggerLastUsed
}

func (sr *immutableRef) updateLastUsedNow() bool {
	if !sr.triggerLastUsed {
		return false
	}
	for r := range sr.refs {
		if r.shouldUpdateLastUsed() {
			return false
		}
	}
	return true
}

func (sr *immutableRef) release(ctx context.Context) error {
	delete(sr.refs, sr)

	if sr.updateLastUsedNow() {
		sr.updateLastUsed()
		if sr.equalMutable != nil {
			sr.equalMutable.triggerLastUsed = true
		}
	}

	if len(sr.refs) == 0 {
		if sr.equalMutable != nil {
			sr.equalMutable.release(ctx)
		} else {
			if err := sr.cm.LeaseManager.Delete(ctx, leases.Lease{ID: sr.viewLeaseID()}); err != nil && !errdefs.IsNotFound(err) {
				return err
			}
			sr.mountCache = nil
		}
	}

	return nil
}

func (sr *immutableRef) Finalize(ctx context.Context) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.finalize(ctx)
}

// caller must hold cacheRecord.mu
func (cr *cacheRecord) finalize(ctx context.Context) error {
	mutable := cr.equalMutable
	if mutable == nil {
		return nil
	}

	_, err := cr.cm.LeaseManager.Create(ctx, func(l *leases.Lease) error {
		l.ID = cr.ID()
		l.Labels = map[string]string{
			"containerd.io/gc.flat": time.Now().UTC().Format(time.RFC3339Nano),
		}
		return nil
	})
	if err != nil {
		if !errors.Is(err, errdefs.ErrAlreadyExists) { // migrator adds leases for everything
			return errors.Wrap(err, "failed to create lease")
		}
	}

	if err := cr.cm.LeaseManager.AddResource(ctx, leases.Lease{ID: cr.ID()}, leases.Resource{
		ID:   cr.getSnapshotID(),
		Type: "snapshots/" + cr.cm.Snapshotter.Name(),
	}); err != nil {
		cr.cm.LeaseManager.Delete(context.TODO(), leases.Lease{ID: cr.ID()})
		return errors.Wrapf(err, "failed to add snapshot %s to lease", cr.getSnapshotID())
	}

	if err := cr.cm.Snapshotter.Commit(ctx, cr.getSnapshotID(), mutable.getSnapshotID()); err != nil {
		cr.cm.LeaseManager.Delete(context.TODO(), leases.Lease{ID: cr.ID()})
		return errors.Wrapf(err, "failed to commit %s to %s during finalize", mutable.getSnapshotID(), cr.getSnapshotID())
	}
	cr.mountCache = nil

	mutable.dead = true
	go func() {
		cr.cm.mu.Lock()
		defer cr.cm.mu.Unlock()
		if err := mutable.remove(context.TODO(), true); err != nil {
			logrus.Error(err)
		}
	}()

	cr.equalMutable = nil
	cr.clearEqualMutable()
	return cr.commitMetadata()
}

func (sr *mutableRef) shouldUpdateLastUsed() bool {
	return sr.triggerLastUsed
}

func (sr *mutableRef) commit(ctx context.Context) (_ *immutableRef, rerr error) {
	if !sr.mutable || len(sr.refs) == 0 {
		return nil, errors.Wrapf(errInvalid, "invalid mutable ref %p", sr)
	}

	id := identity.NewID()
	md, _ := sr.cm.getMetadata(id)
	rec := &cacheRecord{
		mu:            sr.mu,
		cm:            sr.cm,
		parentRefs:    sr.parentRefs.clone(),
		equalMutable:  sr,
		refs:          make(map[ref]struct{}),
		cacheMetadata: md,
	}

	if descr := sr.GetDescription(); descr != "" {
		if err := md.queueDescription(descr); err != nil {
			return nil, err
		}
	}

	if err := initializeMetadata(rec.cacheMetadata, rec.parentRefs); err != nil {
		return nil, err
	}

	sr.cm.records[id] = rec

	if err := sr.commitMetadata(); err != nil {
		return nil, err
	}

	md.queueCommitted(true)
	md.queueSize(sizeUnknown)
	md.queueSnapshotID(id)
	md.setEqualMutable(sr.ID())
	if err := md.commitMetadata(); err != nil {
		return nil, err
	}

	ref := rec.ref(true, sr.descHandlers, nil)
	sr.equalImmutable = ref
	return ref, nil
}

func (sr *mutableRef) Mount(ctx context.Context, readonly bool, s session.Group) (_ snapshot.Mountable, rerr error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.mountCache != nil {
		if readonly {
			return setReadonly(sr.mountCache), nil
		}
		return sr.mountCache, nil
	}

	var mnt snapshot.Mountable
	if sr.cm.Snapshotter.Name() == "stargz" && sr.layerParent != nil {
		if err := sr.layerParent.withRemoteSnapshotLabelsStargzMode(ctx, s, func() {
			mnt, rerr = sr.mount(ctx, s)
		}); err != nil {
			return nil, err
		}
	} else {
		mnt, rerr = sr.mount(ctx, s)
	}
	if rerr != nil {
		return nil, rerr
	}

	// Make the mounts sharable. We don't do this for immutableRef mounts because
	// it requires the raw []mount.Mount for computing diff on overlayfs.
	mnt = sr.cm.mountPool.setSharable(mnt)
	sr.mountCache = mnt
	if readonly {
		mnt = setReadonly(mnt)
	}
	return mnt, nil
}

func (sr *mutableRef) Commit(ctx context.Context) (ImmutableRef, error) {
	sr.cm.mu.Lock()
	defer sr.cm.mu.Unlock()

	sr.mu.Lock()
	defer sr.mu.Unlock()

	return sr.commit(ctx)
}

func (sr *mutableRef) Release(ctx context.Context) error {
	sr.cm.mu.Lock()
	defer sr.cm.mu.Unlock()

	sr.mu.Lock()
	defer sr.mu.Unlock()

	return sr.release(ctx)
}

func (sr *mutableRef) release(ctx context.Context) error {
	delete(sr.refs, sr)
	if !sr.HasCachePolicyRetain() {
		if sr.equalImmutable != nil {
			if sr.equalImmutable.HasCachePolicyRetain() {
				if sr.shouldUpdateLastUsed() {
					sr.updateLastUsed()
					sr.triggerLastUsed = false
				}
				return nil
			}
			if err := sr.equalImmutable.remove(ctx, false); err != nil {
				return err
			}
		}
		return sr.remove(ctx, true)
	}
	if sr.shouldUpdateLastUsed() {
		sr.updateLastUsed()
		sr.triggerLastUsed = false
	}
	return nil
}

func setReadonly(mounts snapshot.Mountable) snapshot.Mountable {
	return &readOnlyMounter{mounts}
}

type readOnlyMounter struct {
	snapshot.Mountable
}

func (m *readOnlyMounter) Mount() ([]mount.Mount, func() error, error) {
	mounts, release, err := m.Mountable.Mount()
	if err != nil {
		return nil, nil, err
	}
	for i, m := range mounts {
		if m.Type == "overlay" {
			mounts[i].Options = readonlyOverlay(m.Options)
			continue
		}
		opts := make([]string, 0, len(m.Options))
		for _, opt := range m.Options {
			if opt != "rw" {
				opts = append(opts, opt)
			}
		}
		opts = append(opts, "ro")
		mounts[i].Options = opts
	}
	return mounts, release, nil
}

func readonlyOverlay(opt []string) []string {
	out := make([]string, 0, len(opt))
	upper := ""
	for _, o := range opt {
		if strings.HasPrefix(o, "upperdir=") {
			upper = strings.TrimPrefix(o, "upperdir=")
		} else if !strings.HasPrefix(o, "workdir=") {
			out = append(out, o)
		}
	}
	if upper != "" {
		for i, o := range out {
			if strings.HasPrefix(o, "lowerdir=") {
				out[i] = "lowerdir=" + upper + ":" + strings.TrimPrefix(o, "lowerdir=")
			}
		}
	}
	return out
}

func newSharableMountPool(tmpdirRoot string) (sharableMountPool, error) {
	if tmpdirRoot != "" {
		if err := os.MkdirAll(tmpdirRoot, 0700); err != nil {
			return sharableMountPool{}, errors.Wrap(err, "failed to prepare mount pool")
		}
		// If tmpdirRoot is specified, remove existing mounts to avoid conflict.
		files, err := os.ReadDir(tmpdirRoot)
		if err != nil {
			return sharableMountPool{}, errors.Wrap(err, "failed to read mount pool")
		}
		for _, file := range files {
			if file.IsDir() {
				dir := filepath.Join(tmpdirRoot, file.Name())
				bklog.G(context.Background()).Debugf("cleaning up existing temporary mount %q", dir)
				if err := mount.Unmount(dir, 0); err != nil {
					if mounted, merr := mountinfo.Mounted(dir); merr != nil || mounted {
						bklog.G(context.Background()).WithError(err).WithError(merr).
							WithField("mounted", mounted).Warnf("failed to unmount existing temporary mount %q", dir)
						continue
					}
				}
				if err := os.Remove(dir); err != nil {
					bklog.G(context.Background()).WithError(err).Warnf("failed to remove existing temporary mount %q", dir)
				}
			}
		}
	}
	return sharableMountPool{tmpdirRoot}, nil
}

type sharableMountPool struct {
	tmpdirRoot string
}

func (p sharableMountPool) setSharable(mounts snapshot.Mountable) snapshot.Mountable {
	return &sharableMountable{Mountable: mounts, mountPoolRoot: p.tmpdirRoot}
}

// sharableMountable allows sharing underlying (possibly writable) mounts among callers.
// This is useful to share writable overlayfs mounts.
//
// NOTE: Mount() method doesn't return the underlying mount configuration (e.g. overlayfs mounts)
//
//	instead it always return bind mounts of the temporary mount point. So if the caller
//	needs to inspect the underlying mount configuration (e.g. for optimized differ for
//	overlayfs), this wrapper shouldn't be used.
type sharableMountable struct {
	snapshot.Mountable

	count         int32
	mu            sync.Mutex
	mountPoolRoot string

	curMounts     []mount.Mount
	curMountPoint string
	curRelease    func() error
}

func (sm *sharableMountable) Mount() (_ []mount.Mount, _ func() error, retErr error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.curMounts == nil {
		mounts, release, err := sm.Mountable.Mount()
		if err != nil {
			return nil, nil, err
		}
		defer func() {
			if retErr != nil {
				release()
			}
		}()
		var isOverlay bool
		for _, m := range mounts {
			if m.Type == "overlay" {
				isOverlay = true
				break
			}
		}
		if !isOverlay {
			// Don't need temporary mount wrapper for non-overlayfs mounts
			return mounts, release, nil
		}
		dir, err := os.MkdirTemp(sm.mountPoolRoot, "buildkit")
		if err != nil {
			return nil, nil, err
		}
		defer func() {
			if retErr != nil {
				os.Remove(dir)
			}
		}()
		if userns.RunningInUserNS() {
			mounts, err = rootlessmountopts.FixUp(mounts)
			if err != nil {
				return nil, nil, err
			}
		}
		if err := mount.All(mounts, dir); err != nil {
			return nil, nil, err
		}
		defer func() {
			if retErr != nil {
				mount.Unmount(dir, 0)
			}
		}()
		sm.curMounts = []mount.Mount{
			{
				Source: dir,
				Type:   "bind",
				Options: []string{
					"rw",
					"rbind",
				},
			},
		}
		sm.curMountPoint = dir
		sm.curRelease = release
	}

	mounts := make([]mount.Mount, len(sm.curMounts))
	copy(mounts, sm.curMounts)

	sm.count++
	return mounts, func() error {
		sm.mu.Lock()
		defer sm.mu.Unlock()

		sm.count--
		if sm.count < 0 {
			if v := os.Getenv("BUILDKIT_DEBUG_PANIC_ON_ERROR"); v == "1" {
				panic("release of released mount")
			}
		} else if sm.count > 0 {
			return nil
		}

		// no mount exist. release the current mount.
		sm.curMounts = nil
		if err := mount.Unmount(sm.curMountPoint, 0); err != nil {
			return err
		}
		if err := sm.curRelease(); err != nil {
			return err
		}
		return os.Remove(sm.curMountPoint)
	}, nil
}
