package plugin

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/docker/pkg/system"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const ociPluginNameKey = "com.docker.plugin.ref.name"

func ociNewManifest() ocispecv1.Manifest {
	return ocispecv1.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		Config: ocispecv1.Descriptor{
			MediaType: schema2.MediaTypePluginConfig,
		},
	}
}

func ociWriteBlob(root string, dgst digest.Digest, r io.Reader) (int64, error) {
	base := filepath.Join(root, "blobs", dgst.Algorithm().String())
	if err := os.MkdirAll(base, 0755); err != nil {
		return 0, errors.Wrap(err, "error creating blob tree")
	}

	fileName := filepath.Join(base, dgst.Hex())
	f, err := os.Create(fileName)
	if err != nil {
		return 0, errors.Wrap(err, "error creating blob file")
	}
	defer f.Close()

	size, err := io.Copy(f, r)
	if err != nil {
		return 0, errors.Wrap(err, "error writing blob to file")
	}

	if err := system.Chtimes(fileName, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
		return 0, errors.Wrap(err, "error resseting times file mod times")
	}
	return size, nil
}

func ociWriteLayout(root string) error {
	layout := &ocispecv1.ImageLayout{
		Version: ocispecv1.ImageLayoutVersion,
	}

	fileName := filepath.Join(root, ocispecv1.ImageLayoutFile)
	data, err := json.Marshal(layout)
	if err != nil {
		return errors.Wrap(err, "error marshaling oci layout")
	}

	if err := ioutil.WriteFile(fileName, data, 644); err != nil {
		return errors.Wrap(err, "error writing oci layout file")
	}

	if err := system.Chtimes(fileName, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
		return errors.Wrap(err, "error resetting mod times on oci layout file")
	}
	return nil
}

func ociWriteIndex(root string, manifests []ocispecv1.Descriptor) error {
	fileName := filepath.Join(root, "index.json")
	index := &ocispecv1.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		Manifests: manifests,
	}

	f, err := os.Create(fileName)
	if err != nil {
		return errors.Wrap(err, "error creating oci index file")
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(index); err != nil {
		return errors.Wrap(err, "error writing oci index")
	}

	if err := system.Chtimes(fileName, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
		return errors.Wrap(err, "error resetting mod times on oci index file")
	}
	return nil
}

type ociImageBundleV1 struct {
	root string
}

func (b *ociImageBundleV1) GetIndex() (ocispecv1.Index, error) {
	var index ocispecv1.Index

	f, err := os.Open(filepath.Join(b.root, "index.json"))
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

	f, err := b.openBlob(d.Digest)
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

func (b *ociImageBundleV1) openBlob(d digest.Digest) (io.ReadCloser, error) {
	f, err := os.Open(filepath.Join(b.root, "blobs", d.Algorithm().String(), d.Hex()))
	if err != nil {
		return nil, errors.Wrap(err, "error opening blob")
	}
	return f, nil
}
