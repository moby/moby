package plugin

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/pkg/system"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

const ociPluginNameKey = "com.docker.plugin.ref.name"

type ociImageBundleV1 struct {
	root string
}

type ociWalkFn func(ocispecv1.Manifest, map[string]string) error

func (b *ociImageBundleV1) Walk(ctx context.Context, fn ociWalkFn) error {
	idx, err := b.GetIndex()
	if err != nil {
		return err
	}

	for _, md := range idx.Manifests {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		m, err := b.ReadManifest(md)
		if err != nil {
			return err
		}
		if err := fn(m, md.Annotations); err != nil {
			return err
		}
	}
	return nil
}

func (b *ociImageBundleV1) GetIndex() (ocispecv1.Index, error) {
	var index ocispecv1.Index

	p, err := symlink.FollowSymlinkInScope(filepath.Join(b.root, "index.json"), b.root)
	if err != nil {
		return index, errors.Wrap(err, "error in index path")
	}
	f, err := os.Open(p)
	if err != nil {
		return index, errors.Wrap(err, "error opening oci index")
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&index); err != nil {
		return index, errors.Wrap(err, "error reading oci index")
	}
	return index, nil
}

func (b *ociImageBundleV1) ReadManifest(d ocispecv1.Descriptor) (ocispecv1.Manifest, error) {
	var m ocispecv1.Manifest

	f, err := b.Store().Get(d.Digest)
	if err != nil {
		return m, errors.Wrap(err, "error reading manifest")
	}
	defer f.Close()

	digester := digest.Canonical.Digester()
	rdr := io.TeeReader(f, digester.Hash())

	if err := json.NewDecoder(rdr).Decode(&m); err != nil {
		return m, errors.Wrap(err, "error decoding manifest")
	}

	if d.Digest != digester.Digest() {
		return m, errDigestMismatch
	}
	return m, nil
}

func (b *ociImageBundleV1) Store() blobstore {
	return &basicBlobStore{filepath.Join(b.root, "blobs")}
}

func (b *ociImageBundleV1) AddBlob(r io.Reader) (ocispecv1.Descriptor, error) {
	blob, err := b.Store().New()
	if err != nil {
		return ocispecv1.Descriptor{}, errors.Wrap(err, "error creating blob")
	}
	defer blob.Close()

	size, err := io.Copy(blob, r)
	if err != nil {
		return ocispecv1.Descriptor{}, errors.Wrap(err, "error writing blob")
	}

	dgst, err := blob.Commit()
	if err != nil {
		return ocispecv1.Descriptor{}, errors.Wrap(err, "error committing blob")
	}

	return ocispecv1.Descriptor{
		Digest: dgst,
		Size:   size,
	}, nil
}

func (b *ociImageBundleV1) Commit(manifests []ocispecv1.Descriptor) error {
	index := &ocispecv1.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		Manifests: manifests,
	}

	fileName := filepath.Join(b.root, "index.json")
	f, err := os.Create(fileName)
	if err != nil {
		return errors.Wrap(err, "error creating oci index file")
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(index); err != nil {
		return errors.Wrap(err, "error writing oci index")
	}

	layout := &ocispecv1.ImageLayout{
		Version: ocispecv1.ImageLayoutVersion,
	}

	fileName = filepath.Join(b.root, ocispecv1.ImageLayoutFile)
	data, err := json.Marshal(layout)
	if err != nil {
		return errors.Wrap(err, "error marshaling oci layout")
	}

	if err := ioutil.WriteFile(fileName, data, 644); err != nil {
		return errors.Wrap(err, "error writing oci layout file")
	}

	os.RemoveAll(filepath.Join(b.root, "blobs", "tmp"))

	// change all mod times to unix epoch for consistent hashing
	err = filepath.Walk(b.root, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if err := system.Chtimes(p, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
			return errors.Wrapf(err, "error resetting mod times on %s", p)
		}
		return nil
	})
	return err
}
