package snapshot

import (
	"context"
	"strconv"

	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/pkg/userns"
	"github.com/containerd/containerd/snapshots"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/pkg/errors"
)

// hardlinkMergeSnapshotters are the names of snapshotters that support merges implemented by
// creating "hardlink farms" where non-directory objects are hard-linked into the merged tree
// from their parent snapshots.
var hardlinkMergeSnapshotters = map[string]struct{}{
	"native":    {},
	"overlayfs": {},
}

// overlayBasedSnapshotters are the names of snapshotter that use overlay mounts, which
// enables optimizations such as skipping the base layer when doing a hardlink merge.
var overlayBasedSnapshotters = map[string]struct{}{
	"overlayfs": {},
	"stargz":    {},
}

type Diff struct {
	Lower string
	Upper string
}

type MergeSnapshotter interface {
	Snapshotter
	// Merge creates a snapshot whose contents are the provided diffs applied onto one
	// another in the provided order, starting from scratch. The diffs are calculated
	// the same way that diffs are calculated during exports, which ensures that the
	// result of merging these diffs looks the same as exporting the diffs as layer
	// blobs and unpacking them as an image.
	//
	// Each key in the provided diffs is expected to be a committed snapshot. The
	// snapshot created by Merge is also committed.
	//
	// The size of a merged snapshot (as returned by the Usage method) depends on the merge
	// implementation. Implementations using hardlinks to create merged views will take up
	// less space than those that use copies, for example.
	Merge(ctx context.Context, key string, diffs []Diff, opts ...snapshots.Opt) error
}

type mergeSnapshotter struct {
	Snapshotter
	lm leases.Manager

	// Whether we should try to implement merges by hardlinking between underlying directories
	tryCrossSnapshotLink bool

	// Whether the optimization of preparing on top of base layers is supported (see Merge method).
	skipBaseLayers bool

	// Whether we should use the "user.*" namespace when writing overlay xattrs. If false,
	// "trusted.*" is used instead.
	userxattr bool
}

func NewMergeSnapshotter(ctx context.Context, sn Snapshotter, lm leases.Manager) MergeSnapshotter {
	name := sn.Name()
	_, tryCrossSnapshotLink := hardlinkMergeSnapshotters[name]
	_, overlayBased := overlayBasedSnapshotters[name]

	skipBaseLayers := overlayBased // default to skipping base layer for overlay-based snapshotters
	var userxattr bool
	if overlayBased && userns.RunningInUserNS() {
		// When using an overlay-based snapshotter, if we are running rootless on a pre-5.11
		// kernel, we will not have userxattr. This results in opaque xattrs not being visible
		// to us and thus breaking the overlay-optimized differ.
		var err error
		userxattr, err = needsUserXAttr(ctx, sn, lm)
		if err != nil {
			bklog.G(ctx).Debugf("failed to check user xattr: %v", err)
			tryCrossSnapshotLink = false
			skipBaseLayers = false
		} else {
			tryCrossSnapshotLink = tryCrossSnapshotLink && userxattr
			// Disable skipping base layers when in pre-5.11 rootless mode. Skipping the base layers
			// necessitates the ability to set opaque xattrs sometimes, which only works in 5.11+
			// kernels that support userxattr.
			skipBaseLayers = userxattr
		}
	}

	return &mergeSnapshotter{
		Snapshotter:          sn,
		lm:                   lm,
		tryCrossSnapshotLink: tryCrossSnapshotLink,
		skipBaseLayers:       skipBaseLayers,
		userxattr:            userxattr,
	}
}

func (sn *mergeSnapshotter) Merge(ctx context.Context, key string, diffs []Diff, opts ...snapshots.Opt) error {
	var baseKey string
	if sn.skipBaseLayers {
		// Overlay-based snapshotters can skip the base snapshot of the merge (if one exists) and just use it as the
		// parent of the merge snapshot. Other snapshotters will start empty (with baseKey set to "").
		// Find the baseKey by following the chain of diffs for as long as it follows the pattern of the current lower
		// being the parent of the current upper and equal to the previous upper, i.e.:
		// Diff("", A) -> Diff(A, B) -> Diff(B, C), etc.
		var baseIndex int
		for i, diff := range diffs {
			var parentKey string
			if diff.Upper != "" {
				info, err := sn.Stat(ctx, diff.Upper)
				if err != nil {
					return err
				}
				parentKey = info.Parent
			}
			if parentKey != diff.Lower {
				break
			}
			if diff.Lower != baseKey {
				break
			}
			baseKey = diff.Upper
			baseIndex = i + 1
		}
		diffs = diffs[baseIndex:]
	}

	ctx, done, err := leaseutil.WithLease(ctx, sn.lm, leaseutil.MakeTemporary)
	if err != nil {
		return errors.Wrap(err, "failed to create temporary lease for view mounts during merge")
	}
	defer done(context.TODO())

	// Make the snapshot that will be merged into
	prepareKey := identity.NewID()
	if err := sn.Prepare(ctx, prepareKey, baseKey); err != nil {
		return errors.Wrapf(err, "failed to prepare %q", key)
	}
	applyMounts, err := sn.Mounts(ctx, prepareKey)
	if err != nil {
		return errors.Wrapf(err, "failed to get mounts of %q", key)
	}

	usage, err := sn.diffApply(ctx, applyMounts, diffs...)
	if err != nil {
		return errors.Wrap(err, "failed to apply diffs")
	}
	if err := sn.Commit(ctx, key, prepareKey, withMergeUsage(usage)); err != nil {
		return errors.Wrapf(err, "failed to commit %q", key)
	}
	return nil
}

func (sn *mergeSnapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	// If key was created by Merge, we may need to use the annotated mergeUsage key as
	// the snapshotter's usage method is wrong when hardlinks are used to create the merge.
	if info, err := sn.Stat(ctx, key); err != nil {
		return snapshots.Usage{}, err
	} else if usage, ok, err := mergeUsageOf(info); err != nil {
		return snapshots.Usage{}, err
	} else if ok {
		return usage, nil
	}
	return sn.Snapshotter.Usage(ctx, key)
}

// mergeUsage{Size,Inodes}Label hold the correct usage calculations for diffApplyMerges, for which the builtin usage
// is wrong because it can't account for hardlinks made across immutable snapshots
const mergeUsageSizeLabel = "buildkit.mergeUsageSize"
const mergeUsageInodesLabel = "buildkit.mergeUsageInodes"

func withMergeUsage(usage snapshots.Usage) snapshots.Opt {
	return snapshots.WithLabels(map[string]string{
		mergeUsageSizeLabel:   strconv.Itoa(int(usage.Size)),
		mergeUsageInodesLabel: strconv.Itoa(int(usage.Inodes)),
	})
}

func mergeUsageOf(info snapshots.Info) (usage snapshots.Usage, ok bool, rerr error) {
	if info.Labels == nil {
		return snapshots.Usage{}, false, nil
	}
	if str, ok := info.Labels[mergeUsageSizeLabel]; ok {
		i, err := strconv.Atoi(str)
		if err != nil {
			return snapshots.Usage{}, false, err
		}
		usage.Size = int64(i)
	}
	if str, ok := info.Labels[mergeUsageInodesLabel]; ok {
		i, err := strconv.Atoi(str)
		if err != nil {
			return snapshots.Usage{}, false, err
		}
		usage.Inodes = int64(i)
	}
	return usage, true, nil
}
