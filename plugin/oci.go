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
