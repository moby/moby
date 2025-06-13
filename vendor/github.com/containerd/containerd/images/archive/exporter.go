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
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/containerd/log"
	"github.com/containerd/platforms"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/labels"
)

type exportOptions struct {
	manifests          []ocispec.Descriptor
	platform           platforms.MatchComparer
	allPlatforms       bool
	skipDockerManifest bool
	blobRecordOptions  blobRecordOptions
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

// WithImages adds multiples images to the exported archive.
func WithImages(imgs []images.Image) ExportOpt {
	return func(ctx context.Context, o *exportOptions) error {
		for _, img := range imgs {
			img.Target.Annotations = addNameAnnotation(img.Name, img.Target.Annotations)
			o.manifests = append(o.manifests, img.Target)
		}

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

// BlobFilter returns false if the blob should not be included in the archive.
type BlobFilter func(ocispec.Descriptor) bool

// WithBlobFilter specifies BlobFilter.
func WithBlobFilter(f BlobFilter) ExportOpt {
	return func(ctx context.Context, o *exportOptions) error {
		o.blobRecordOptions.blobFilter = f
		return nil
	}
}

// WithSkipNonDistributableBlobs excludes non-distributable blobs such as Windows base layers.
func WithSkipNonDistributableBlobs() ExportOpt {
	f := func(desc ocispec.Descriptor) bool {
		return !images.IsNonDistributable(desc.MediaType)
	}
	return WithBlobFilter(f)
}

// WithSkipMissing excludes blobs referenced by manifests if not all blobs
// would be included in the archive.
// The manifest itself is excluded only if it's not present locally.
// This allows to export multi-platform images if not all platforms are present
// while still persisting the multi-platform index.
func WithSkipMissing(store content.InfoReaderProvider) ExportOpt {
	return func(ctx context.Context, o *exportOptions) error {
		o.blobRecordOptions.childrenHandler = images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) (subdescs []ocispec.Descriptor, err error) {
			children, err := images.Children(ctx, store, desc)
			if !images.IsManifestType(desc.MediaType) {
				return children, err
			}

			if err != nil {
				// If manifest itself is missing, skip it from export.
				if errdefs.IsNotFound(err) {
					return nil, images.ErrSkipDesc
				}
				return nil, err
			}

			// Don't export manifest descendants if any of them doesn't exist.
			for _, child := range children {
				exists, err := content.Exists(ctx, store, child)
				if err != nil {
					return nil, err
				}

				// If any child is missing, only export the manifest, but don't export its descendants.
				if !exists {
					return nil, nil
				}
			}
			return children, nil
		})
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

func copySourceLabels(ctx context.Context, infoProvider content.InfoProvider, desc ocispec.Descriptor) (ocispec.Descriptor, error) {
	info, err := infoProvider.Info(ctx, desc.Digest)
	if err != nil {
		return desc, err
	}
	for k, v := range info.Labels {
		if strings.HasPrefix(k, labels.LabelDistributionSource) {
			if desc.Annotations == nil {
				desc.Annotations = map[string]string{k: v}
			} else {
				desc.Annotations[k] = v
			}
		}
	}
	return desc, nil
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
	}

	manifests := make([]ocispec.Descriptor, 0, len(eo.manifests))
	if infoProvider, ok := store.(content.InfoProvider); ok {
		for _, desc := range eo.manifests {
			d, err := copySourceLabels(ctx, infoProvider, desc)
			if err != nil {
				log.G(ctx).WithError(err).WithField("desc", desc).Warn("failed to copy distribution.source labels")
				continue
			}
			manifests = append(manifests, d)
		}
	} else {
		manifests = append(manifests, eo.manifests...)
	}

	algorithms := map[string]struct{}{}
	dManifests := map[digest.Digest]*exportManifest{}
	resolvedIndex := map[digest.Digest]digest.Digest{}
	for _, desc := range manifests {
		if images.IsManifestType(desc.MediaType) {
			mt, ok := dManifests[desc.Digest]
			if !ok {
				// TODO(containerd): Skip if already added
				r, err := getRecords(ctx, store, desc, algorithms, &eo.blobRecordOptions)
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
			if name != "" {
				mt.names = append(mt.names, name)
			}
		} else if images.IsIndexType(desc.MediaType) {
			d, ok := resolvedIndex[desc.Digest]
			if !ok {
				if err := desc.Digest.Validate(); err != nil {
					return err
				}
				records = append(records, blobRecord(store, desc, &eo.blobRecordOptions))

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

					r, err := getRecords(ctx, store, m, algorithms, &eo.blobRecordOptions)
					if err != nil {
						return err
					}

					records = append(records, r...)
				}

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
					return fmt.Errorf("no manifest found for platform: %w", errdefs.ErrNotFound)
				}
				resolvedIndex[desc.Digest] = d
			}
			if d != "" {
				if name := desc.Annotations[images.AnnotationImageName]; name != "" {
					mt := dManifests[d]
					mt.names = append(mt.names, name)
				}

			}
		} else {
			return fmt.Errorf("only manifests may be exported: %w", errdefs.ErrInvalidArgument)
		}
	}

	records = append(records, ociIndexRecord(manifests))

	if !eo.skipDockerManifest && len(dManifests) > 0 {
		tr, err := manifestsRecord(ctx, store, dManifests)
		if err != nil {
			return fmt.Errorf("unable to create manifests file: %w", err)
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

func getRecords(ctx context.Context, store content.Provider, desc ocispec.Descriptor, algorithms map[string]struct{}, brOpts *blobRecordOptions) ([]tarRecord, error) {
	var records []tarRecord
	exportHandler := func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if err := desc.Digest.Validate(); err != nil {
			return nil, err
		}
		records = append(records, blobRecord(store, desc, brOpts))
		algorithms[desc.Digest.Algorithm().String()] = struct{}{}
		return nil, nil
	}

	childrenHandler := brOpts.childrenHandler
	if childrenHandler == nil {
		childrenHandler = images.ChildrenHandler(store)
	}

	handlers := images.Handlers(
		childrenHandler,
		images.HandlerFunc(exportHandler),
	)

	// Walk sequentially since the number of fetches is likely one and doing in
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

type blobRecordOptions struct {
	blobFilter      BlobFilter
	childrenHandler images.HandlerFunc
}

func blobRecord(cs content.Provider, desc ocispec.Descriptor, opts *blobRecordOptions) tarRecord {
	if opts != nil && opts.blobFilter != nil && !opts.blobFilter(desc) {
		return tarRecord{}
	}
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
				return 0, fmt.Errorf("failed to get reader: %w", err)
			}
			defer r.Close()

			// Verify digest
			dgstr := desc.Digest.Algorithm().Digester()

			n, err := io.Copy(io.MultiWriter(w, dgstr.Hash()), content.NewReader(r))
			if err != nil {
				return 0, fmt.Errorf("failed to copy to tar: %w", err)
			}
			if dgstr.Digest() != desc.Digest {
				return 0, fmt.Errorf("unexpected digest %s copied", dgstr.Digest())
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
		MediaType: ocispec.MediaTypeImageIndex,
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
			return tarRecord{}, fmt.Errorf("invalid manifest %q: %w", m.manifest.Digest, err)
		}

		dgst := manifest.Config.Digest
		if err := dgst.Validate(); err != nil {
			return tarRecord{}, err
		}
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

func writeTar(ctx context.Context, tw *tar.Writer, recordsWithEmpty []tarRecord) error {
	var records []tarRecord
	for _, r := range recordsWithEmpty {
		if r.Header != nil {
			records = append(records, r)
		}
	}
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
				return fmt.Errorf("unexpected copy size for %s", record.Header.Name)
			}
		} else if record.Header.Size > 0 {
			return fmt.Errorf("no content to write to record with non-zero size for %s", record.Header.Name)
		}
	}
	return nil
}
