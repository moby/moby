package blobmapping

import (
	"context"

	"github.com/boltdb/bolt"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/snapshots"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/snapshot"
	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

const blobKey = "blobmapping.blob"

type Opt struct {
	Content       content.Store
	Snapshotter   snapshot.SnapshotterBase
	MetadataStore *metadata.Store
}

type Info struct {
	snapshots.Info
	Blob string
}

type DiffPair struct {
	Blobsum digest.Digest
	DiffID  digest.Digest
}

// this snapshotter keeps an internal mapping between a snapshot and a blob

type Snapshotter struct {
	snapshot.SnapshotterBase
	opt Opt
}

func NewSnapshotter(opt Opt) snapshot.Snapshotter {
	s := &Snapshotter{
		SnapshotterBase: opt.Snapshotter,
		opt:             opt,
	}

	return s
}

// Remove also removes a reference to a blob. If it is a last reference then it deletes it the blob as well
// Remove is not safe to be called concurrently
func (s *Snapshotter) Remove(ctx context.Context, key string) error {
	_, blob, err := s.GetBlob(ctx, key)
	if err != nil {
		return err
	}

	blobs, err := s.opt.MetadataStore.Search(index(blob))
	if err != nil {
		return err
	}

	if err := s.SnapshotterBase.Remove(ctx, key); err != nil {
		return err
	}

	if len(blobs) == 1 && blobs[0].ID() == key { // last snapshot
		if err := s.opt.Content.Delete(ctx, blob); err != nil {
			logrus.Errorf("failed to delete blob %v: %+v", blob, err)
		}
	}
	return nil
}

func (s *Snapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	u, err := s.SnapshotterBase.Usage(ctx, key)
	if err != nil {
		return snapshots.Usage{}, err
	}
	_, blob, err := s.GetBlob(ctx, key)
	if err != nil {
		return u, err
	}
	if blob != "" {
		info, err := s.opt.Content.Info(ctx, blob)
		if err != nil {
			return u, err
		}
		(&u).Add(snapshots.Usage{Size: info.Size, Inodes: 1})
	}
	return u, nil
}

func (s *Snapshotter) GetBlob(ctx context.Context, key string) (digest.Digest, digest.Digest, error) {
	md, _ := s.opt.MetadataStore.Get(key)
	v := md.Get(blobKey)
	if v == nil {
		return "", "", nil
	}
	var blob DiffPair
	if err := v.Unmarshal(&blob); err != nil {
		return "", "", err
	}
	return blob.DiffID, blob.Blobsum, nil
}

// Validates that there is no blob associated with the snapshot.
// Checks that there is a blob in the content store.
// If same blob has already been set then this is a noop.
func (s *Snapshotter) SetBlob(ctx context.Context, key string, diffID, blobsum digest.Digest) error {
	_, err := s.opt.Content.Info(ctx, blobsum)
	if err != nil {
		return err
	}
	md, _ := s.opt.MetadataStore.Get(key)

	v, err := metadata.NewValue(DiffPair{DiffID: diffID, Blobsum: blobsum})
	if err != nil {
		return err
	}
	v.Index = index(blobsum)

	return md.Update(func(b *bolt.Bucket) error {
		return md.SetValue(b, blobKey, v)
	})
}

func index(blob digest.Digest) string {
	return "blobmap::" + blob.String()
}
