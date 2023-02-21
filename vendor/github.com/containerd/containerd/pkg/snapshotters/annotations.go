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

package snapshotters

import (
	"context"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/labels"
	"github.com/containerd/containerd/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// NOTE: The following labels contain "cri" prefix but they are not specific to CRI and
// can be used by non-CRI clients as well for enabling remote snapshotters. We need to
// retain that string for keeping compatibility with snapshotter implementations.
const (
	// TargetRefLabel is a label which contains image reference and will be passed
	// to snapshotters.
	TargetRefLabel = "containerd.io/snapshot/cri.image-ref"
	// TargetManifestDigestLabel is a label which contains manifest digest and will be passed
	// to snapshotters.
	TargetManifestDigestLabel = "containerd.io/snapshot/cri.manifest-digest"
	// TargetLayerDigestLabel is a label which contains layer digest and will be passed
	// to snapshotters.
	TargetLayerDigestLabel = "containerd.io/snapshot/cri.layer-digest"
	// TargetImageLayersLabel is a label which contains layer digests contained in
	// the target image and will be passed to snapshotters for preparing layers in
	// parallel. Skipping some layers is allowed and only affects performance.
	TargetImageLayersLabel = "containerd.io/snapshot/cri.image-layers"
)

// AppendInfoHandlerWrapper makes a handler which appends some basic information
// of images like digests for manifest and their child layers as annotations during unpack.
// These annotations will be passed to snapshotters as labels. These labels will be
// used mainly by remote snapshotters for querying image contents from the remote location.
func AppendInfoHandlerWrapper(ref string) func(f images.Handler) images.Handler {
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
						c.Annotations[TargetRefLabel] = ref
						c.Annotations[TargetLayerDigestLabel] = c.Digest.String()
						c.Annotations[TargetImageLayersLabel] = getLayers(ctx, TargetImageLayersLabel, children[i:], labels.Validate)
						c.Annotations[TargetManifestDigestLabel] = desc.Digest.String()
					}
				}
			}
			return children, nil
		})
	}
}

// getLayers returns comma-separated digests based on the passed list of
// descriptors. The returned list contains as many digests as possible as well
// as meets the label validation.
func getLayers(ctx context.Context, key string, descs []ocispec.Descriptor, validate func(k, v string) error) (layers string) {
	for _, l := range descs {
		if images.IsLayerType(l.MediaType) {
			item := l.Digest.String()
			if layers != "" {
				item = "," + item
			}
			// This avoids the label hits the size limitation.
			if err := validate(key, layers+item); err != nil {
				log.G(ctx).WithError(err).WithField("label", key).WithField("digest", l.Digest.String()).Debug("omitting digest in the layers list")
				break
			}
			layers += item
		}
	}
	return
}
