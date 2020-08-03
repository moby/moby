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
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func migrateChainID(si *metadata.StorageItem, all map[string]*metadata.StorageItem) (digest.Digest, digest.Digest, error) {
	diffID := digest.Digest(getDiffID(si))
	if diffID == "" {
		return "", "", nil
	}
	blobID := digest.Digest(getBlob(si))
	if blobID == "" {
		return "", "", nil
	}
	chainID := digest.Digest(getChainID(si))
	blobChainID := digest.Digest(getBlobChainID(si))

	if chainID != "" && blobChainID != "" {
		return chainID, blobChainID, nil
	}

	chainID = diffID
	blobChainID = digest.FromBytes([]byte(blobID + " " + diffID))

	parent := getParent(si)
	if parent != "" {
		pChainID, pBlobChainID, err := migrateChainID(all[parent], all)
		if err != nil {
			return "", "", err
		}
		chainID = digest.FromBytes([]byte(pChainID + " " + chainID))
		blobChainID = digest.FromBytes([]byte(pBlobChainID + " " + blobChainID))
	}

	queueChainID(si, chainID.String())
	queueBlobChainID(si, blobChainID.String())

	return chainID, blobChainID, si.Commit()
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
		em := getEqualMutable(item)
		if em == "" {
			info, err := s.Stat(ctx, id)
			if err != nil {
				return err
			}
			if info.Kind == snapshots.KindCommitted {
				queueCommitted(item)
			}
			if info.Parent != "" {
				queueParent(item, info.Parent)
			}
		} else {
			queueCommitted(item)
		}
		queueSnapshotID(item, id)
		item.Commit()
	}

	for _, item := range byID {
		em := getEqualMutable(item)
		if em != "" {
			if getParent(item) == "" {
				queueParent(item, getParent(byID[em]))
				item.Commit()
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
		queueDiffID(item, blob.DiffID)
		queueBlob(item, blob.Blobsum)
		queueMediaType(item, images.MediaTypeDockerSchema2LayerGzip)
		if err := item.Commit(); err != nil {
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
			ID:   getSnapshotID(item),
			Type: "snapshots/" + s.Name(),
		}); err != nil {
			return errors.Wrapf(err, "failed to add snapshot %s to lease", item.ID())
		}

		if blobID := getBlob(item); blobID != "" {
			if err := lm.AddResource(ctx, l, leases.Resource{
				ID:   blobID,
				Type: "content",
			}); err != nil {
				return errors.Wrapf(err, "failed to add blob %s to lease", item.ID())
			}
		}
	}

	// remove old root labels
	for _, item := range byID {
		em := getEqualMutable(item)
		if em == "" {
			if _, err := s.Update(ctx, snapshots.Info{
				Name: getSnapshotID(item),
			}, "labels.containerd.io/gc.root"); err != nil {
				if !errors.Is(err, errdefs.ErrNotFound) {
					return err
				}
			}

			if blob := getBlob(item); blob != "" {
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
		logrus.Infof("migrated %s parent:%q snapshot:%v committed:%v blob:%v diffid:%v chainID:%v blobChainID:%v",
			item.ID(), getParent(item), getSnapshotID(item), getCommitted(item), getBlob(item), getDiffID(item), getChainID(item), getBlobChainID(item))
	}

	return nil
}
