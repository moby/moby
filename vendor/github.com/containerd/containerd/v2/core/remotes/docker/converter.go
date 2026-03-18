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

package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/log"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// LegacyConfigMediaType should be replaced by OCI image spec.
//
// More detail: docker/distribution#1622
const LegacyConfigMediaType = "application/octet-stream"

// ConvertManifest changes application/octet-stream to schema2 config media type if need.
//
// NOTE:
// 1. original manifest will be deleted by next gc round.
// 2. don't cover manifest list.
func ConvertManifest(ctx context.Context, store content.Store, desc ocispec.Descriptor) (ocispec.Descriptor, error) {
	if !images.IsManifestType(desc.MediaType) {
		log.G(ctx).Warnf("do nothing for media type: %s", desc.MediaType)
		return desc, nil
	}

	// read manifest data
	mb, err := content.ReadBlob(ctx, store, desc)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to read index data: %w", err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(mb, &manifest); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to unmarshal data into manifest: %w", err)
	}

	// check config media type
	if manifest.Config.MediaType != LegacyConfigMediaType {
		return desc, nil
	}

	manifest.Config.MediaType = images.MediaTypeDockerSchema2Config
	data, err := json.MarshalIndent(manifest, "", "   ")
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// update manifest with gc labels
	desc.Digest = digest.Canonical.FromBytes(data)
	desc.Size = int64(len(data))

	labels := map[string]string{}
	for i, c := range append([]ocispec.Descriptor{manifest.Config}, manifest.Layers...) {
		labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i)] = c.Digest.String()
	}

	ref := remotes.MakeRefKey(ctx, desc)
	if err := content.WriteBlob(ctx, store, ref, bytes.NewReader(data), desc, content.WithLabels(labels)); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to update content: %w", err)
	}
	return desc, nil
}
