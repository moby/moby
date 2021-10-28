package cache

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/snapshots"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/util/bklog"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

func migrateChainID(si *metadata.StorageItem, all map[string]*metadata.StorageItem) (digest.Digest, digest.Digest, error) {
	md := &cacheMetadata{si}
	diffID := md.getDiffID()
	if diffID == "" {
		return "", "", nil
	}
	blobID := md.getBlob()
	if blobID == "" {
		return "", "", nil
	}
	chainID := md.getChainID()
	blobChainID := md.getBlobChainID()

	if chainID != "" && blobChainID != "" {
		return chainID, blobChainID, nil
	}

	chainID = diffID
	blobChainID = digest.FromBytes([]byte(blobID + " " + diffID))

	parent := md.getParent()
	if parent != "" {
		pChainID, pBlobChainID, err := migrateChainID(all[parent], all)
		if err != nil {
			return "", "", err
		}
		chainID = digest.FromBytes([]byte(pChainID + " " + chainID))
		blobChainID = digest.FromBytes([]byte(pBlobChainID + " " + blobChainID))
	}

	md.queueChainID(chainID)
	md.queueBlobChainID(blobChainID)

	return chainID, blobChainID, md.commitMetadata()
}

func MigrateV2(ctx context.Context, from, to string, cs content.Store, s snapshot.Snapshotter, lm leases.Manager) error {
	_, err := os.Stat(to)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return errors.WithStack(err)
		}
	} else {
		return nil
	}

	_, err = os.Stat(from)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return errors.WithStack(err)
		}
		return nil
	}
	tmpPath := to + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return errors.WithStack(err)
	}
	src, err := os.Open(from)
	if err != nil {
		tmpFile.Close()
		return errors.WithStack(err)
	}
	if _, err = io.Copy(tmpFile, src); err != nil {
		tmpFile.Close()
		src.Close()
		return errors.Wrapf(err, "failed to copy db for migration")
	}
	src.Close()
	tmpFile.Close()

	md, err := metadata.NewStore(tmpPath)
	if err != nil {
		return err
	}

	items, err := md.All()
	if err != nil {
		return err
	}

	byID := map[string]*metadata.StorageItem{}
	for _, item := range items {
		byID[item.ID()] = item
	}

	// add committed, parent, snapshot
	for id, item := range byID {
		md := &cacheMetadata{item}
		em := md.getEqualMutable()
		if em == "" {
			info, err := s.Stat(ctx, id)
			if err != nil {
				return err
			}
			if info.Kind == snapshots.KindCommitted {
				md.queueCommitted(true)
			}
			if info.Parent != "" {
				md.queueParent(info.Parent)
			}
		} else {
			md.queueCommitted(true)
		}
		md.queueSnapshotID(id)
		md.commitMetadata()
	}

	for _, item := range byID {
		md := &cacheMetadata{item}
		em := md.getEqualMutable()
		if em != "" {
			if md.getParent() == "" {
				emMd := &cacheMetadata{byID[em]}
				md.queueParent(emMd.getParent())
				md.commitMetadata()
			}
		}
	}

	type diffPair struct {
		Blobsum string
		DiffID  string
	}
	// move diffID, blobsum to new location
	for _, item := range byID {
		v := item.Get("blobmapping.blob")
		if v == nil {
			continue
		}
		var blob diffPair
		if err := v.Unmarshal(&blob); err != nil {
			return errors.WithStack(err)
		}
		if _, err := cs.Info(ctx, digest.Digest(blob.Blobsum)); err != nil {
			continue
		}
		md := &cacheMetadata{item}
		md.queueDiffID(digest.Digest(blob.DiffID))
		md.queueBlob(digest.Digest(blob.Blobsum))
		md.queueMediaType(images.MediaTypeDockerSchema2LayerGzip)
		if err := md.commitMetadata(); err != nil {
			return err
		}

	}

	// calculate new chainid/blobsumid
	for _, item := range byID {
		if _, _, err := migrateChainID(item, byID); err != nil {
			return err
		}
	}

	ctx = context.TODO() // no cancellation allowed pass this point

	// add new leases
	for _, item := range byID {
		md := &cacheMetadata{item}
		l, err := lm.Create(ctx, func(l *leases.Lease) error {
			l.ID = item.ID()
			l.Labels = map[string]string{
				"containerd.io/gc.flat": time.Now().UTC().Format(time.RFC3339Nano),
			}
			return nil
		})
		if err != nil {
			// if we are running the migration twice
			if errors.Is(err, errdefs.ErrAlreadyExists) {
				continue
			}
			return errors.Wrap(err, "failed to create lease")
		}

		if err := lm.AddResource(ctx, l, leases.Resource{
			ID:   md.getSnapshotID(),
			Type: "snapshots/" + s.Name(),
		}); err != nil {
			return errors.Wrapf(err, "failed to add snapshot %s to lease", item.ID())
		}

		if blobID := md.getBlob(); blobID != "" {
			if err := lm.AddResource(ctx, l, leases.Resource{
				ID:   string(blobID),
				Type: "content",
			}); err != nil {
				return errors.Wrapf(err, "failed to add blob %s to lease", item.ID())
			}
		}
	}

	// remove old root labels
	for _, item := range byID {
		md := &cacheMetadata{item}
		em := md.getEqualMutable()
		if em == "" {
			if _, err := s.Update(ctx, snapshots.Info{
				Name: md.getSnapshotID(),
			}, "labels.containerd.io/gc.root"); err != nil {
				if !errors.Is(err, errdefs.ErrNotFound) {
					return err
				}
			}

			if blob := md.getBlob(); blob != "" {
				if _, err := cs.Update(ctx, content.Info{
					Digest: digest.Digest(blob),
				}, "labels.containerd.io/gc.root"); err != nil {
					return err
				}
			}
		}
	}

	// previous implementation can leak views, just clean up all views
	err = s.Walk(ctx, func(ctx context.Context, info snapshots.Info) error {
		if info.Kind == snapshots.KindView {
			if _, err := s.Update(ctx, snapshots.Info{
				Name: info.Name,
			}, "labels.containerd.io/gc.root"); err != nil {
				if !errors.Is(err, errdefs.ErrNotFound) {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// switch to new DB
	if err := md.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, to); err != nil {
		return err
	}

	for _, item := range byID {
		md := &cacheMetadata{item}
		bklog.G(ctx).Infof("migrated %s parent:%q snapshot:%v blob:%v diffid:%v chainID:%v blobChainID:%v",
			item.ID(), md.getParent(), md.getSnapshotID(), md.getBlob(), md.getDiffID(), md.getChainID(), md.getBlobChainID())
	}

	return nil
}
