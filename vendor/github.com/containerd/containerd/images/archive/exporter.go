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

package archive

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"path"
	"sort"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type exportOptions struct {
	manifests          []ocispec.Descriptor
	platform           platforms.MatchComparer
	allPlatforms       bool
	skipDockerManifest bool
}

// ExportOpt defines options for configuring exported descriptors
type ExportOpt func(context.Context, *exportOptions) error

// WithPlatform defines the platform to require manifest lists have
// not exporting all platforms.
// Additionally, platform is used to resolve image configs for
// Docker v1.1, v1.2 format compatibility.
func WithPlatform(p platforms.MatchComparer) ExportOpt {
	return func(ctx context.Context, o *exportOptions) error {
		o.platform = p
		return nil
	}
}

// WithAllPlatforms exports all manifests from a manifest list.
// Missing content will fail the export.
func WithAllPlatforms() ExportOpt {
	return func(ctx context.Context, o *exportOptions) error {
		o.allPlatforms = true
		return nil
	}
}

// WithSkipDockerManifest skips creation of the Docker compatible
// manifest.json file.
func WithSkipDockerManifest() ExportOpt {
	return func(ctx context.Context, o *exportOptions) error {
		o.skipDockerManifest = true
		return nil
	}
}

// WithImage adds the provided images to the exported archive.
func WithImage(is images.Store, name string) ExportOpt {
	return func(ctx context.Context, o *exportOptions) error {
		img, err := is.Get(ctx, name)
		if err != nil {
			return err
		}

		img.Target.Annotations = addNameAnnotation(name, img.Target.Annotations)
		o.manifests = append(o.manifests, img.Target)

		return nil
	}
}

// WithManifest adds a manifest to the exported archive.
// When names are given they will be set on the manifest in the
// exported archive, creating an index record for each name.
// When no names are provided, it is up to caller to put name annotation to
// on the manifest descriptor if needed.
func WithManifest(manifest ocispec.Descriptor, names ...string) ExportOpt {
	return func(ctx context.Context, o *exportOptions) error {
		if len(names) == 0 {
			o.manifests = append(o.manifests, manifest)
		}
		for _, name := range names {
			mc := manifest
			mc.Annotations = addNameAnnotation(name, manifest.Annotations)
			o.manifests = append(o.manifests, mc)
		}

		return nil
	}
}

func addNameAnnotation(name string, base map[string]string) map[string]string {
	annotations := map[string]string{}
	for k, v := range base {
		annotations[k] = v
	}

	annotations[images.AnnotationImageName] = name
	annotations[ocispec.AnnotationRefName] = ociReferenceName(name)

	return annotations
}

// Export implements Exporter.
func Export(ctx context.Context, store content.Provider, writer io.Writer, opts ...ExportOpt) error {
	var eo exportOptions
	for _, opt := range opts {
		if err := opt(ctx, &eo); err != nil {
			return err
		}
	}

	records := []tarRecord{
		ociLayoutFile(""),
		ociIndexRecord(eo.manifests),
	}

	algorithms := map[string]struct{}{}
	dManifests := map[digest.Digest]*exportManifest{}
	resolvedIndex := map[digest.Digest]digest.Digest{}
	for _, desc := range eo.manifests {
		switch desc.MediaType {
		case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
			mt, ok := dManifests[desc.Digest]
			if !ok {
				// TODO(containerd): Skip if already added
				r, err := getRecords(ctx, store, desc, algorithms)
				if err != nil {
					return err
				}
				records = append(records, r...)

				mt = &exportManifest{
					manifest: desc,
				}
				dManifests[desc.Digest] = mt
			}

			name := desc.Annotations[images.AnnotationImageName]
			if name != "" && !eo.skipDockerManifest {
				mt.names = append(mt.names, name)
			}
		case images.MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex:
			d, ok := resolvedIndex[desc.Digest]
			if !ok {
				records = append(records, blobRecord(store, desc))

				p, err := content.ReadBlob(ctx, store, desc)
				if err != nil {
					return err
				}

				var index ocispec.Index
				if err := json.Unmarshal(p, &index); err != nil {
					return err
				}

				var manifests []ocispec.Descriptor
				for _, m := range index.Manifests {
					if eo.platform != nil {
						if m.Platform == nil || eo.platform.Match(*m.Platform) {
							manifests = append(manifests, m)
						} else if !eo.allPlatforms {
							continue
						}
					}

					r, err := getRecords(ctx, store, m, algorithms)
					if err != nil {
						return err
					}

					records = append(records, r...)
				}

				if !eo.skipDockerManifest {
					if len(manifests) >= 1 {
						if len(manifests) > 1 {
							sort.SliceStable(manifests, func(i, j int) bool {
								if manifests[i].Platform == nil {
									return false
								}
								if manifests[j].Platform == nil {
									return true
								}
								return eo.platform.Less(*manifests[i].Platform, *manifests[j].Platform)
							})
						}
						d = manifests[0].Digest
						dManifests[d] = &exportManifest{
							manifest: manifests[0],
						}
					} else if eo.platform != nil {
						return errors.Wrap(errdefs.ErrNotFound, "no manifest found for platform")
					}
				}
				resolvedIndex[desc.Digest] = d
			}
			if d != "" {
				if name := desc.Annotations[images.AnnotationImageName]; name != "" {
					mt := dManifests[d]
					mt.names = append(mt.names, name)
				}

			}
		default:
			return errors.Wrap(errdefs.ErrInvalidArgument, "only manifests may be exported")
		}
	}

	if len(dManifests) > 0 {
		tr, err := manifestsRecord(ctx, store, dManifests)
		if err != nil {
			return errors.Wrap(err, "unable to create manifests file")
		}

		records = append(records, tr)
	}

	if len(algorithms) > 0 {
		records = append(records, directoryRecord("blobs/", 0755))
		for alg := range algorithms {
			records = append(records, directoryRecord("blobs/"+alg+"/", 0755))
		}
	}

	tw := tar.NewWriter(writer)
	defer tw.Close()
	return writeTar(ctx, tw, records)
}

func getRecords(ctx context.Context, store content.Provider, desc ocispec.Descriptor, algorithms map[string]struct{}) ([]tarRecord, error) {
	var records []tarRecord
	exportHandler := func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		records = append(records, blobRecord(store, desc))
		algorithms[desc.Digest.Algorithm().String()] = struct{}{}
		return nil, nil
	}

	childrenHandler := images.ChildrenHandler(store)

	handlers := images.Handlers(
		childrenHandler,
		images.HandlerFunc(exportHandler),
	)

	// Walk sequentially since the number of fetchs is likely one and doing in
	// parallel requires locking the export handler
	if err := images.Walk(ctx, handlers, desc); err != nil {
		return nil, err
	}

	return records, nil
}

type tarRecord struct {
	Header *tar.Header
	CopyTo func(context.Context, io.Writer) (int64, error)
}

func blobRecord(cs content.Provider, desc ocispec.Descriptor) tarRecord {
	path := path.Join("blobs", desc.Digest.Algorithm().String(), desc.Digest.Encoded())
	return tarRecord{
		Header: &tar.Header{
			Name:     path,
			Mode:     0444,
			Size:     desc.Size,
			Typeflag: tar.TypeReg,
		},
		CopyTo: func(ctx context.Context, w io.Writer) (int64, error) {
			r, err := cs.ReaderAt(ctx, desc)
			if err != nil {
				return 0, errors.Wrap(err, "failed to get reader")
			}
			defer r.Close()

			// Verify digest
			dgstr := desc.Digest.Algorithm().Digester()

			n, err := io.Copy(io.MultiWriter(w, dgstr.Hash()), content.NewReader(r))
			if err != nil {
				return 0, errors.Wrap(err, "failed to copy to tar")
			}
			if dgstr.Digest() != desc.Digest {
				return 0, errors.Errorf("unexpected digest %s copied", dgstr.Digest())
			}
			return n, nil
		},
	}
}

func directoryRecord(name string, mode int64) tarRecord {
	return tarRecord{
		Header: &tar.Header{
			Name:     name,
			Mode:     mode,
			Typeflag: tar.TypeDir,
		},
	}
}

func ociLayoutFile(version string) tarRecord {
	if version == "" {
		version = ocispec.ImageLayoutVersion
	}
	layout := ocispec.ImageLayout{
		Version: version,
	}

	b, err := json.Marshal(layout)
	if err != nil {
		panic(err)
	}

	return tarRecord{
		Header: &tar.Header{
			Name:     ocispec.ImageLayoutFile,
			Mode:     0444,
			Size:     int64(len(b)),
			Typeflag: tar.TypeReg,
		},
		CopyTo: func(ctx context.Context, w io.Writer) (int64, error) {
			n, err := w.Write(b)
			return int64(n), err
		},
	}

}

func ociIndexRecord(manifests []ocispec.Descriptor) tarRecord {
	index := ocispec.Index{
		Versioned: ocispecs.Versioned{
			SchemaVersion: 2,
		},
		Manifests: manifests,
	}

	b, err := json.Marshal(index)
	if err != nil {
		panic(err)
	}

	return tarRecord{
		Header: &tar.Header{
			Name:     "index.json",
			Mode:     0644,
			Size:     int64(len(b)),
			Typeflag: tar.TypeReg,
		},
		CopyTo: func(ctx context.Context, w io.Writer) (int64, error) {
			n, err := w.Write(b)
			return int64(n), err
		},
	}
}

type exportManifest struct {
	manifest ocispec.Descriptor
	names    []string
}

func manifestsRecord(ctx context.Context, store content.Provider, manifests map[digest.Digest]*exportManifest) (tarRecord, error) {
	mfsts := make([]struct {
		Config   string
		RepoTags []string
		Layers   []string
	}, len(manifests))

	var i int
	for _, m := range manifests {
		p, err := content.ReadBlob(ctx, store, m.manifest)
		if err != nil {
			return tarRecord{}, err
		}

		var manifest ocispec.Manifest
		if err := json.Unmarshal(p, &manifest); err != nil {
			return tarRecord{}, err
		}
		if err := manifest.Config.Digest.Validate(); err != nil {
			return tarRecord{}, errors.Wrapf(err, "invalid manifest %q", m.manifest.Digest)
		}

		dgst := manifest.Config.Digest
		mfsts[i].Config = path.Join("blobs", dgst.Algorithm().String(), dgst.Encoded())
		for _, l := range manifest.Layers {
			path := path.Join("blobs", l.Digest.Algorithm().String(), l.Digest.Encoded())
			mfsts[i].Layers = append(mfsts[i].Layers, path)
		}

		for _, name := range m.names {
			nname, err := familiarizeReference(name)
			if err != nil {
				return tarRecord{}, err
			}

			mfsts[i].RepoTags = append(mfsts[i].RepoTags, nname)
		}

		i++
	}

	b, err := json.Marshal(mfsts)
	if err != nil {
		return tarRecord{}, err
	}

	return tarRecord{
		Header: &tar.Header{
			Name:     "manifest.json",
			Mode:     0644,
			Size:     int64(len(b)),
			Typeflag: tar.TypeReg,
		},
		CopyTo: func(ctx context.Context, w io.Writer) (int64, error) {
			n, err := w.Write(b)
			return int64(n), err
		},
	}, nil
}

func writeTar(ctx context.Context, tw *tar.Writer, records []tarRecord) error {
	sort.Slice(records, func(i, j int) bool {
		return records[i].Header.Name < records[j].Header.Name
	})

	var last string
	for _, record := range records {
		if record.Header.Name == last {
			continue
		}
		last = record.Header.Name
		if err := tw.WriteHeader(record.Header); err != nil {
			return err
		}
		if record.CopyTo != nil {
			n, err := record.CopyTo(ctx, tw)
			if err != nil {
				return err
			}
			if n != record.Header.Size {
				return errors.Errorf("unexpected copy size for %s", record.Header.Name)
			}
		} else if record.Header.Size > 0 {
			return errors.Errorf("no content to write to record with non-zero size for %s", record.Header.Name)
		}
	}
	return nil
}
