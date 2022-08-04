/*
 * Copyright (c) 2022. Ant Group. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package label

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/labels"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// AppendLabelsHandlerWrapper returns a function which can wrap a handler by appending
// image's basic information to each layer descriptor as annotations during unpack.
// These annotations will be passed to this nydus snapshotter as labels.
func AppendLabelsHandlerWrapper(ref string) func(f images.Handler) images.Handler {
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
						var layers string
						for _, l := range children[i:] {
							if images.IsLayerType(l.MediaType) {
								ls := fmt.Sprintf("%s,", l.Digest.String())
								// This avoids the label hits the size limitation.
								// Skipping layers is allowed here and only affects performance.
								if err := labels.Validate(CRIImageLayers, layers+ls); err != nil {
									break
								}
								layers += ls
							}
						}
						c.Annotations[CRIImageLayers] = strings.TrimSuffix(layers, ",")
						c.Annotations[CRIImageRef] = ref
						c.Annotations[CRILayerDigest] = c.Digest.String()
						c.Annotations[CRIManifestDigest] = desc.Digest.String()
					}
				}
			}
			return children, nil
		})
	}
}
