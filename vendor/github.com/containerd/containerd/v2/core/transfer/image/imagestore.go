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

package image

import (
	"context"
	"fmt"

	"github.com/containerd/typeurl/v2"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/containerd/containerd/api/types"
	transfertypes "github.com/containerd/containerd/api/types/transfer"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/images/archive"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/streaming"
	"github.com/containerd/containerd/v2/core/transfer"
	"github.com/containerd/containerd/v2/core/transfer/plugins"
	"github.com/containerd/errdefs"
	"github.com/containerd/platforms"
)

func init() {
	// TODO: Move this to separate package?
	plugins.Register(&transfertypes.ImageStore{}, &Store{}) // TODO: Rename ImageStoreDestination
}

type Store struct {
	imageName     string
	imageLabels   map[string]string
	platforms     []ocispec.Platform
	allMetadata   bool
	labelMap      func(ocispec.Descriptor) []string
	manifestLimit int

	// extraReferences are used to store or lookup multiple references
	extraReferences []Reference

	unpacks []transfer.UnpackConfiguration
}

// Reference is used to create or find a reference for an image
type Reference struct {
	Name string

	// IsPrefix determines whether the Name should be considered
	// a prefix (without tag or digest).
	// For lookup, this may allow matching multiple tags.
	// For store, this must have a tag or digest added.
	IsPrefix bool

	// AllowOverwrite allows overwriting or ignoring the name if
	// another reference is provided (such as through an annotation).
	// Only used if IsPrefix is true.
	AllowOverwrite bool

	// AddDigest adds the manifest digest to the reference.
	// For lookup, this allows matching tags with any digest.
	// For store, this allows adding the digest to the name.
	// Only used if IsPrefix is true.
	AddDigest bool

	// SkipNamedDigest only considers digest references which do not
	// have a non-digested named reference.
	// For lookup, this will deduplicate digest references when there is a named match.
	// For store, this only adds this digest reference when there is no matching full
	// name reference from the prefix.
	// Only used if IsPrefix is true.
	SkipNamedDigest bool
}

// StoreOpt defines options when configuring an image store source or destination
type StoreOpt func(*Store)

// WithImageLabels are the image labels to apply to a new image
func WithImageLabels(labels map[string]string) StoreOpt {
	return func(s *Store) {
		s.imageLabels = labels
	}
}

// WithPlatforms specifies which platforms to fetch content for
func WithPlatforms(p ...ocispec.Platform) StoreOpt {
	return func(s *Store) {
		s.platforms = append(s.platforms, p...)
	}
}

// WithManifestLimit defines the max number of manifests to fetch
func WithManifestLimit(limit int) StoreOpt {
	return func(s *Store) {
		s.manifestLimit = limit
	}
}

func WithAllMetadata(s *Store) {
	s.allMetadata = true
}

// WithNamedPrefix uses a named prefix to references images which only have a tag name
// reference in the annotation or check full references annotations against. Images
// with no reference resolved from matching annotations will not be stored.
// - name: image name prefix to append a tag to or check full name references with
// - allowOverwrite: allows the tag to be overwritten by full name reference inside
// the image which does not have name as the prefix
func WithNamedPrefix(name string, allowOverwrite bool) StoreOpt {
	ref := Reference{
		Name:           name,
		IsPrefix:       true,
		AllowOverwrite: allowOverwrite,
	}
	return func(s *Store) {
		s.extraReferences = append(s.extraReferences, ref)
	}
}

// WithDigestRef uses a named prefix to references images which only have a tag name
// reference in the annotation or check full references annotations against and
// additionally may add a digest reference. Images with no references resolved
// from matching annotations may be stored by digest.
// - name: image name prefix to append a tag to or check full name references with
// - allowOverwrite: allows the tag to be overwritten by full name reference inside
// the image which does not have name as the prefix
// - skipNamed: is set if no digest reference should be created if a named reference
// is successfully resolved from the annotations.
func WithDigestRef(name string, allowOverwrite bool, skipNamed bool) StoreOpt {
	ref := Reference{
		Name:            name,
		IsPrefix:        true,
		AllowOverwrite:  allowOverwrite,
		AddDigest:       true,
		SkipNamedDigest: skipNamed,
	}
	return func(s *Store) {
		s.extraReferences = append(s.extraReferences, ref)
	}
}

func WithExtraReference(name string) StoreOpt {
	ref := Reference{
		Name: name,
	}
	return func(s *Store) {
		s.extraReferences = append(s.extraReferences, ref)
	}
}

// WithUnpack specifies a platform to unpack for and an optional snapshotter to use
func WithUnpack(p ocispec.Platform, snapshotter string) StoreOpt {
	return func(s *Store) {
		s.unpacks = append(s.unpacks, transfer.UnpackConfiguration{
			Platform:    p,
			Snapshotter: snapshotter,
		})
	}
}

// NewStore creates a new image store source or Destination
func NewStore(image string, opts ...StoreOpt) *Store {
	s := &Store{
		imageName: image,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (is *Store) String() string {
	return fmt.Sprintf("Local Image Store (%s)", is.imageName)
}

func (is *Store) ImageFilter(h images.HandlerFunc, cs content.Store) images.HandlerFunc {
	var p platforms.MatchComparer
	if len(is.platforms) == 0 {
		p = platforms.All
	} else {
		p = platforms.Ordered(is.platforms...)
	}
	h = images.SetChildrenMappedLabels(cs, h, is.labelMap)
	if is.allMetadata {
		// Filter manifests by platforms but allow to handle manifest
		// and configuration for not-target platforms
		h = remotes.FilterManifestByPlatformHandler(h, p)
	} else {
		// Filter children by platforms if specified.
		h = images.FilterPlatforms(h, p)
	}

	// Sort and limit manifests if a finite number is needed
	if is.manifestLimit > 0 {
		h = images.LimitManifests(h, p, is.manifestLimit)
	}
	return h
}

func (is *Store) Store(ctx context.Context, desc ocispec.Descriptor, store images.Store) ([]images.Image, error) {
	var imgs []images.Image

	// If import ref type, store references from annotation or prefix
	if refSource, ok := desc.Annotations["io.containerd.import.ref-source"]; ok {
		switch refSource {
		case "annotation":
			for _, ref := range is.extraReferences {
				// Only use prefix references for annotation matching
				if !ref.IsPrefix {
					continue
				}

				var nameT func(string) string
				if ref.AllowOverwrite {
					nameT = archive.AddRefPrefix(ref.Name)
				} else {
					nameT = archive.FilterRefPrefix(ref.Name)
				}
				name := imageName(desc.Annotations, nameT)

				if name == "" {
					// If digested, add digest reference
					if ref.AddDigest {
						imgs = append(imgs, images.Image{
							Name:   fmt.Sprintf("%s@%s", ref.Name, desc.Digest),
							Target: desc,
							Labels: is.imageLabels,
						})
					}
					continue
				}

				imgs = append(imgs, images.Image{
					Name:   name,
					Target: desc,
					Labels: is.imageLabels,
				})

				// If a named reference was found and SkipNamedDigest is true, do
				// not use this reference
				if ref.AddDigest && !ref.SkipNamedDigest {
					imgs = append(imgs, images.Image{
						Name:   fmt.Sprintf("%s@%s", ref.Name, desc.Digest),
						Target: desc,
						Labels: is.imageLabels,
					})
				}
			}
		default:
			return nil, fmt.Errorf("ref source not supported: %w", errdefs.ErrInvalidArgument)
		}
		delete(desc.Annotations, "io.containerd.import.ref-source")
	} else {
		if is.imageName != "" {
			imgs = append(imgs, images.Image{
				Name:   is.imageName,
				Target: desc,
				Labels: is.imageLabels,
			})
		}

		// If extra references, store all complete references (skip prefixes)
		for _, ref := range is.extraReferences {
			if ref.IsPrefix {
				continue
			}
			name := ref.Name
			if ref.AddDigest {
				name = fmt.Sprintf("%s@%s", name, desc.Digest)
			}
			imgs = append(imgs, images.Image{
				Name:   name,
				Target: desc,
				Labels: is.imageLabels,
			})
		}
	}

	if len(imgs) == 0 {
		return nil, fmt.Errorf("no image name found: %w", errdefs.ErrNotFound)
	}

	for i := 0; i < len(imgs); {
		if created, err := store.Create(ctx, imgs[i]); err != nil {
			if !errdefs.IsAlreadyExists(err) {
				return nil, err
			}

			updated, err := store.Update(ctx, imgs[i])
			if err != nil {
				// if image was removed, try create again
				if errdefs.IsNotFound(err) {
					// Keep trying same image
					continue
				}
				return nil, err
			}

			imgs[i] = updated
		} else {
			imgs[i] = created
		}

		i++
	}

	return imgs, nil
}

func (is *Store) Get(ctx context.Context, store images.Store) (images.Image, error) {
	return store.Get(ctx, is.imageName)
}

func (is *Store) Lookup(ctx context.Context, store images.Store) ([]images.Image, error) {
	var imgs []images.Image
	if is.imageName != "" {
		img, err := store.Get(ctx, is.imageName)
		if err != nil {
			return nil, err
		}
		imgs = append(imgs, img)
	}
	for _, ref := range is.extraReferences {
		if ref.IsPrefix {
			return nil, fmt.Errorf("prefix lookup on export not implemented: %w", errdefs.ErrNotImplemented)
		}
		img, err := store.Get(ctx, ref.Name)
		if err != nil {
			return nil, err
		}
		imgs = append(imgs, img)
	}
	return imgs, nil
}

func (is *Store) Platforms() []ocispec.Platform {
	return is.platforms
}

func (is *Store) UnpackPlatforms() []transfer.UnpackConfiguration {
	unpacks := make([]transfer.UnpackConfiguration, len(is.unpacks))
	for i, uc := range is.unpacks {
		unpacks[i].Snapshotter = uc.Snapshotter
		unpacks[i].Platform = uc.Platform
	}
	return unpacks
}

func (is *Store) MarshalAny(context.Context, streaming.StreamCreator) (typeurl.Any, error) {
	s := &transfertypes.ImageStore{
		Name:            is.imageName,
		Labels:          is.imageLabels,
		ManifestLimit:   uint32(is.manifestLimit),
		AllMetadata:     is.allMetadata,
		Platforms:       types.OCIPlatformToProto(is.platforms),
		ExtraReferences: referencesToProto(is.extraReferences),
		Unpacks:         unpackToProto(is.unpacks),
	}
	return typeurl.MarshalAny(s)
}

func (is *Store) UnmarshalAny(ctx context.Context, sm streaming.StreamGetter, a typeurl.Any) error {
	var s transfertypes.ImageStore
	if err := typeurl.UnmarshalTo(a, &s); err != nil {
		return err
	}

	is.imageName = s.Name
	is.imageLabels = s.Labels
	is.manifestLimit = int(s.ManifestLimit)
	is.allMetadata = s.AllMetadata
	is.platforms = types.OCIPlatformFromProto(s.Platforms)
	is.extraReferences = referencesFromProto(s.ExtraReferences)
	is.unpacks = unpackFromProto(s.Unpacks)

	return nil
}

func referencesToProto(references []Reference) []*transfertypes.ImageReference {
	ir := make([]*transfertypes.ImageReference, len(references))
	for i := range references {
		r := transfertypes.ImageReference{
			Name:            references[i].Name,
			IsPrefix:        references[i].IsPrefix,
			AllowOverwrite:  references[i].AllowOverwrite,
			AddDigest:       references[i].AddDigest,
			SkipNamedDigest: references[i].SkipNamedDigest,
		}

		ir[i] = &r
	}
	return ir
}

func referencesFromProto(references []*transfertypes.ImageReference) []Reference {
	or := make([]Reference, len(references))
	for i := range references {
		or[i].Name = references[i].Name
		or[i].IsPrefix = references[i].IsPrefix
		or[i].AllowOverwrite = references[i].AllowOverwrite
		or[i].AddDigest = references[i].AddDigest
		or[i].SkipNamedDigest = references[i].SkipNamedDigest
	}
	return or
}
func unpackToProto(uc []transfer.UnpackConfiguration) []*transfertypes.UnpackConfiguration {
	auc := make([]*transfertypes.UnpackConfiguration, len(uc))
	for i := range uc {
		p := types.Platform{
			OS:           uc[i].Platform.OS,
			Architecture: uc[i].Platform.Architecture,
			Variant:      uc[i].Platform.Variant,
		}
		auc[i] = &transfertypes.UnpackConfiguration{
			Platform:    &p,
			Snapshotter: uc[i].Snapshotter,
		}
	}
	return auc
}

func unpackFromProto(auc []*transfertypes.UnpackConfiguration) []transfer.UnpackConfiguration {
	uc := make([]transfer.UnpackConfiguration, len(auc))
	for i := range auc {
		uc[i].Snapshotter = auc[i].Snapshotter
		if auc[i].Platform != nil {
			uc[i].Platform.OS = auc[i].Platform.OS
			uc[i].Platform.Architecture = auc[i].Platform.Architecture
			uc[i].Platform.Variant = auc[i].Platform.Variant
		}
	}
	return uc
}

func imageName(annotations map[string]string, cleanup func(string) string) string {
	name := annotations[images.AnnotationImageName]
	if name != "" {
		if cleanup != nil {
			// containerd reference name should be full reference and not
			// modified, if it is incomplete or does not match a specified
			// prefix, do not use the reference
			if cleanName := cleanup(name); cleanName != name {
				name = ""
			}
		}
		return name
	}
	name = annotations[ocispec.AnnotationRefName]
	if name != "" {
		if cleanup != nil {
			name = cleanup(name)
		}
	}
	return name
}
