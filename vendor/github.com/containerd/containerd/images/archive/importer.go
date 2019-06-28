/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package archive provides a Docker and OCI compatible importer
package archive

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"path"

	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ImportIndex imports an index from a tar archive image bundle
// - implements Docker v1.1, v1.2 and OCI v1.
// - prefers OCI v1 when provided
// - creates OCI index for Docker formats
// - normalizes Docker references and adds as OCI ref name
//      e.g. alpine:latest -> docker.io/library/alpine:latest
// - existing OCI reference names are untouched
// - TODO: support option to compress layers on ingest
func ImportIndex(ctx context.Context, store content.Store, reader io.Reader) (ocispec.Descriptor, error) {
	var (
		tr = tar.NewReader(reader)

		ociLayout ocispec.ImageLayout
		mfsts     []struct {
			Config   string
			RepoTags []string
			Layers   []string
		}
		symlinks = make(map[string]string)
		blobs    = make(map[string]ocispec.Descriptor)
	)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ocispec.Descriptor{}, err
		}
		if hdr.Typeflag == tar.TypeSymlink {
			symlinks[hdr.Name] = path.Join(path.Dir(hdr.Name), hdr.Linkname)
		}

		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			if hdr.Typeflag != tar.TypeDir {
				log.G(ctx).WithField("file", hdr.Name).Debug("file type ignored")
			}
			continue
		}

		hdrName := path.Clean(hdr.Name)
		if hdrName == ocispec.ImageLayoutFile {
			if err = onUntarJSON(tr, &ociLayout); err != nil {
				return ocispec.Descriptor{}, errors.Wrapf(err, "untar oci layout %q", hdr.Name)
			}
		} else if hdrName == "manifest.json" {
			if err = onUntarJSON(tr, &mfsts); err != nil {
				return ocispec.Descriptor{}, errors.Wrapf(err, "untar manifest %q", hdr.Name)
			}
		} else {
			dgst, err := onUntarBlob(ctx, tr, store, hdr.Size, "tar-"+hdrName)
			if err != nil {
				return ocispec.Descriptor{}, errors.Wrapf(err, "failed to ingest %q", hdr.Name)
			}

			blobs[hdrName] = ocispec.Descriptor{
				Digest: dgst,
				Size:   hdr.Size,
			}
		}
	}

	// If OCI layout was given, interpret the tar as an OCI layout.
	// When not provided, the layout of the tar will be interpretted
	// as Docker v1.1 or v1.2.
	if ociLayout.Version != "" {
		if ociLayout.Version != ocispec.ImageLayoutVersion {
			return ocispec.Descriptor{}, errors.Errorf("unsupported OCI version %s", ociLayout.Version)
		}

		idx, ok := blobs["index.json"]
		if !ok {
			return ocispec.Descriptor{}, errors.Errorf("missing index.json in OCI layout %s", ocispec.ImageLayoutVersion)
		}

		idx.MediaType = ocispec.MediaTypeImageIndex
		return idx, nil
	}

	if mfsts == nil {
		return ocispec.Descriptor{}, errors.Errorf("unrecognized image format")
	}

	for name, linkname := range symlinks {
		desc, ok := blobs[linkname]
		if !ok {
			return ocispec.Descriptor{}, errors.Errorf("no target for symlink layer from %q to %q", name, linkname)
		}
		blobs[name] = desc
	}

	idx := ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
	}
	for _, mfst := range mfsts {
		config, ok := blobs[mfst.Config]
		if !ok {
			return ocispec.Descriptor{}, errors.Errorf("image config %q not found", mfst.Config)
		}
		config.MediaType = ocispec.MediaTypeImageConfig

		layers, err := resolveLayers(ctx, store, mfst.Layers, blobs)
		if err != nil {
			return ocispec.Descriptor{}, errors.Wrap(err, "failed to resolve layers")
		}

		manifest := ocispec.Manifest{
			Versioned: specs.Versioned{
				SchemaVersion: 2,
			},
			Config: config,
			Layers: layers,
		}

		desc, err := writeManifest(ctx, store, manifest, ocispec.MediaTypeImageManifest)
		if err != nil {
			return ocispec.Descriptor{}, errors.Wrap(err, "write docker manifest")
		}

		platforms, err := images.Platforms(ctx, store, desc)
		if err != nil {
			return ocispec.Descriptor{}, errors.Wrap(err, "unable to resolve platform")
		}
		if len(platforms) > 0 {
			// Only one platform can be resolved from non-index manifest,
			// The platform can only come from the config included above,
			// if the config has no platform it can be safely omitted.
			desc.Platform = &platforms[0]
		}

		if len(mfst.RepoTags) == 0 {
			idx.Manifests = append(idx.Manifests, desc)
		} else {
			// Add descriptor per tag
			for _, ref := range mfst.RepoTags {
				mfstdesc := desc

				normalized, err := normalizeReference(ref)
				if err != nil {
					return ocispec.Descriptor{}, err
				}

				mfstdesc.Annotations = map[string]string{
					ocispec.AnnotationRefName: normalized,
				}

				idx.Manifests = append(idx.Manifests, mfstdesc)
			}
		}
	}

	return writeManifest(ctx, store, idx, ocispec.MediaTypeImageIndex)
}

func onUntarJSON(r io.Reader, j interface{}) error {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, j)
}

func onUntarBlob(ctx context.Context, r io.Reader, store content.Ingester, size int64, ref string) (digest.Digest, error) {
	dgstr := digest.Canonical.Digester()

	if err := content.WriteBlob(ctx, store, ref, io.TeeReader(r, dgstr.Hash()), ocispec.Descriptor{Size: size}); err != nil {
		return "", err
	}

	return dgstr.Digest(), nil
}

func resolveLayers(ctx context.Context, store content.Store, layerFiles []string, blobs map[string]ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	var layers []ocispec.Descriptor
	for _, f := range layerFiles {
		desc, ok := blobs[f]
		if !ok {
			return nil, errors.Errorf("layer %q not found", f)
		}

		// Open blob, resolve media type
		ra, err := store.ReaderAt(ctx, desc)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open %q (%s)", f, desc.Digest)
		}
		s, err := compression.DecompressStream(content.NewReader(ra))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to detect compression for %q", f)
		}
		if s.GetCompression() == compression.Uncompressed {
			// TODO: Support compressing and writing back to content store
			desc.MediaType = ocispec.MediaTypeImageLayer
		} else {
			desc.MediaType = ocispec.MediaTypeImageLayerGzip
		}
		s.Close()

		layers = append(layers, desc)
	}
	return layers, nil
}

func writeManifest(ctx context.Context, cs content.Ingester, manifest interface{}, mediaType string) (ocispec.Descriptor, error) {
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(manifestBytes),
		Size:      int64(len(manifestBytes)),
	}
	if err := content.WriteBlob(ctx, cs, "manifest-"+desc.Digest.String(), bytes.NewReader(manifestBytes), desc); err != nil {
		return ocispec.Descriptor{}, err
	}

	return desc, nil
}
