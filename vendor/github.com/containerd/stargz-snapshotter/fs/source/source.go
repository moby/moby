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

package source

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/labels"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/stargz-snapshotter/fs/config"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// GetSources is a function for converting snapshot labels into typed blob sources
// information. This package defines a default converter which provides source
// information based on some labels but implementations aren't required to use labels.
// Implementations are allowed to return several sources (registry config + image refs)
// about the blob.
type GetSources func(labels map[string]string) (source []Source, err error)

// RegistryHosts returns a list of registries that provides the specified image.
type RegistryHosts func(reference.Spec) ([]docker.RegistryHost, error)

// Source is a typed blob source information. This contains information about
// a blob stored in registries and some contexts of the blob.
type Source struct {

	// Hosts is a registry configuration where this blob is stored.
	Hosts RegistryHosts

	// Name is an image reference which contains this blob.
	Name reference.Spec

	// Target is a descriptor of this blob.
	Target ocispec.Descriptor

	// Manifest is an image manifest which contains the blob. This will
	// be used by the filesystem to pre-resolve some layers contained in
	// the manifest.
	// Currently, only layer digests (Manifest.Layers.Digest) will be used.
	Manifest ocispec.Manifest
}

const (
	// targetRefLabel is a label which contains image reference.
	targetRefLabel = "containerd.io/snapshot/remote/stargz.reference"

	// targetDigestLabel is a label which contains layer digest.
	targetDigestLabel = "containerd.io/snapshot/remote/stargz.digest"

	// targetImageLayersLabel is a label which contains layer digests contained in
	// the target image.
	targetImageLayersLabel = "containerd.io/snapshot/remote/stargz.layers"

	// targetImageURLsLabelPrefix is a label prefix which constructs a map from the layer index to
	// urls of the layer descriptor.
	targetImageURLsLabelPrefix = "containerd.io/snapshot/remote/urls."

	// targetURsLLabel is a label which contains layer URL. This is only used to pass URL from containerd
	// to snapshotter.
	targetURLsLabel = "containerd.io/snapshot/remote/urls"
)

// FromDefaultLabels returns a function for converting snapshot labels to
// source information based on labels.
func FromDefaultLabels(hosts RegistryHosts) GetSources {
	return func(labels map[string]string) ([]Source, error) {
		refStr, ok := labels[targetRefLabel]
		if !ok {
			return nil, fmt.Errorf("reference hasn't been passed")
		}
		refspec, err := reference.Parse(refStr)
		if err != nil {
			return nil, err
		}

		digestStr, ok := labels[targetDigestLabel]
		if !ok {
			return nil, fmt.Errorf("digest hasn't been passed")
		}
		target, err := digest.Parse(digestStr)
		if err != nil {
			return nil, err
		}

		var neighboringLayers []ocispec.Descriptor
		if l, ok := labels[targetImageLayersLabel]; ok {
			layersStr := strings.Split(l, ",")
			for i, l := range layersStr {
				d, err := digest.Parse(l)
				if err != nil {
					return nil, err
				}
				if d.String() != target.String() {
					desc := ocispec.Descriptor{Digest: d}
					if urls, ok := labels[targetImageURLsLabelPrefix+fmt.Sprintf("%d", i)]; ok {
						desc.URLs = strings.Split(urls, ",")
					}
					neighboringLayers = append(neighboringLayers, desc)
				}
			}
		}

		targetDesc := ocispec.Descriptor{
			Digest:      target,
			Annotations: labels,
		}
		if targetURLs, ok := labels[targetURLsLabel]; ok {
			targetDesc.URLs = append(targetDesc.URLs, strings.Split(targetURLs, ",")...)
		}

		return []Source{
			{
				Hosts:    hosts,
				Name:     refspec,
				Target:   targetDesc,
				Manifest: ocispec.Manifest{Layers: append([]ocispec.Descriptor{targetDesc}, neighboringLayers...)},
			},
		}, nil
	}
}

// AppendDefaultLabelsHandlerWrapper makes a handler which appends image's basic
// information to each layer descriptor as annotations during unpack. These
// annotations will be passed to this remote snapshotter as labels and used to
// construct source information.
func AppendDefaultLabelsHandlerWrapper(ref string, prefetchSize int64) func(f images.Handler) images.Handler {
	return func(f images.Handler) images.Handler {
		return images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			children, err := f.Handle(ctx, desc)
			if err != nil {
				return nil, err
			}
			switch desc.MediaType {
			case ocispec.MediaTypeImageManifest, images.MediaTypeDockerSchema2Manifest:
				for i := range children {
					c := &children[i]
					if images.IsLayerType(c.MediaType) {
						if c.Annotations == nil {
							c.Annotations = make(map[string]string)
						}
						c.Annotations[targetRefLabel] = ref
						c.Annotations[targetDigestLabel] = c.Digest.String()
						var layers string
						for i, l := range children[i:] {
							if images.IsLayerType(l.MediaType) {
								ls := fmt.Sprintf("%s,", l.Digest.String())
								// This avoids the label hits the size limitation.
								// Skipping layers is allowed here and only affects performance.
								if err := labels.Validate(targetImageLayersLabel, layers+ls); err != nil {
									break
								}
								layers += ls

								// Store URLs of the neighbouring layer as well.
								urlsKey := targetImageURLsLabelPrefix + fmt.Sprintf("%d", i)
								c.Annotations[urlsKey] = appendWithValidation(urlsKey, l.URLs)
							}
						}
						c.Annotations[targetImageLayersLabel] = strings.TrimSuffix(layers, ",")
						c.Annotations[config.TargetPrefetchSizeLabel] = fmt.Sprintf("%d", prefetchSize)

						// store URL in annotation to let containerd to pass it to the snapshotter
						c.Annotations[targetURLsLabel] = appendWithValidation(targetURLsLabel, c.URLs)
					}
				}
			}
			return children, nil
		})
	}
}

func appendWithValidation(key string, values []string) string {
	var v string
	for _, u := range values {
		s := fmt.Sprintf("%s,", u)
		if err := labels.Validate(key, v+s); err != nil {
			break
		}
		v += s
	}
	return strings.TrimSuffix(v, ",")
}
