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

package images

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/containerd/containerd/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// mediatype definitions for image components handled in containerd.
//
// oci components are generally referenced directly, although we may centralize
// here for clarity.
const (
	MediaTypeDockerSchema2Layer            = "application/vnd.docker.image.rootfs.diff.tar"
	MediaTypeDockerSchema2LayerForeign     = "application/vnd.docker.image.rootfs.foreign.diff.tar"
	MediaTypeDockerSchema2LayerGzip        = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	MediaTypeDockerSchema2LayerForeignGzip = "application/vnd.docker.image.rootfs.foreign.diff.tar.gzip"
	MediaTypeDockerSchema2Config           = "application/vnd.docker.container.image.v1+json"
	MediaTypeDockerSchema2Manifest         = "application/vnd.docker.distribution.manifest.v2+json"
	MediaTypeDockerSchema2ManifestList     = "application/vnd.docker.distribution.manifest.list.v2+json"

	// Checkpoint/Restore Media Types

	MediaTypeContainerd1Checkpoint               = "application/vnd.containerd.container.criu.checkpoint.criu.tar"
	MediaTypeContainerd1CheckpointPreDump        = "application/vnd.containerd.container.criu.checkpoint.predump.tar"
	MediaTypeContainerd1Resource                 = "application/vnd.containerd.container.resource.tar"
	MediaTypeContainerd1RW                       = "application/vnd.containerd.container.rw.tar"
	MediaTypeContainerd1CheckpointConfig         = "application/vnd.containerd.container.checkpoint.config.v1+proto"
	MediaTypeContainerd1CheckpointOptions        = "application/vnd.containerd.container.checkpoint.options.v1+proto"
	MediaTypeContainerd1CheckpointRuntimeName    = "application/vnd.containerd.container.checkpoint.runtime.name"
	MediaTypeContainerd1CheckpointRuntimeOptions = "application/vnd.containerd.container.checkpoint.runtime.options+proto"

	// MediaTypeDockerSchema1Manifest is the legacy Docker schema1 manifest
	MediaTypeDockerSchema1Manifest = "application/vnd.docker.distribution.manifest.v1+prettyjws"

	// Encrypted media types

	MediaTypeImageLayerEncrypted     = ocispec.MediaTypeImageLayer + "+encrypted"
	MediaTypeImageLayerGzipEncrypted = ocispec.MediaTypeImageLayerGzip + "+encrypted"

	// In-toto attestation
	MediaTypeInToto = "application/vnd.in-toto+json"
)

// DiffCompression returns the compression as defined by the layer diff media
// type. For Docker media types without compression, "unknown" is returned to
// indicate that the media type may be compressed. If the media type is not
// recognized as a layer diff, then it returns errdefs.ErrNotImplemented
func DiffCompression(ctx context.Context, mediaType string) (string, error) {
	base, ext := parseMediaTypes(mediaType)
	switch base {
	case MediaTypeDockerSchema2Layer, MediaTypeDockerSchema2LayerForeign:
		if len(ext) > 0 {
			// Type is wrapped
			return "", nil
		}
		// These media types may have been compressed but failed to
		// use the correct media type. The decompression function
		// should detect and handle this case.
		return "unknown", nil
	case MediaTypeDockerSchema2LayerGzip, MediaTypeDockerSchema2LayerForeignGzip:
		if len(ext) > 0 {
			// Type is wrapped
			return "", nil
		}
		return "gzip", nil
	case ocispec.MediaTypeImageLayer, ocispec.MediaTypeImageLayerNonDistributable: //nolint:staticcheck // Non-distributable layers are deprecated
		if len(ext) > 0 {
			switch ext[len(ext)-1] {
			case "gzip":
				return "gzip", nil
			case "zstd":
				return "zstd", nil
			}
		}
		return "", nil
	default:
		return "", fmt.Errorf("unrecognised mediatype %s: %w", mediaType, errdefs.ErrNotImplemented)
	}
}

// parseMediaTypes splits the media type into the base type and
// an array of sorted extensions
func parseMediaTypes(mt string) (mediaType string, suffixes []string) {
	if mt == "" {
		return "", []string{}
	}
	mediaType, ext, ok := strings.Cut(mt, "+")
	if !ok {
		return mediaType, []string{}
	}

	// Splitting the extensions following the mediatype "(+)gzip+encrypted".
	// We expect this to be a limited list, so add an arbitrary limit (50).
	//
	// Note that DiffCompression is only using the last element, so perhaps we
	// should split on the last "+" only.
	suffixes = strings.SplitN(ext, "+", 50)
	sort.Strings(suffixes)
	return mediaType, suffixes
}

// IsNonDistributable returns true if the media type is non-distributable.
func IsNonDistributable(mt string) bool {
	return strings.HasPrefix(mt, "application/vnd.oci.image.layer.nondistributable.") ||
		strings.HasPrefix(mt, "application/vnd.docker.image.rootfs.foreign.")
}

// IsLayerType returns true if the media type is a layer
func IsLayerType(mt string) bool {
	if strings.HasPrefix(mt, "application/vnd.oci.image.layer.") {
		return true
	}

	// Parse Docker media types, strip off any + suffixes first
	switch base, _ := parseMediaTypes(mt); base {
	case MediaTypeDockerSchema2Layer, MediaTypeDockerSchema2LayerGzip,
		MediaTypeDockerSchema2LayerForeign, MediaTypeDockerSchema2LayerForeignGzip:
		return true
	}
	return false
}

// IsDockerType returns true if the media type has "application/vnd.docker." prefix
func IsDockerType(mt string) bool {
	return strings.HasPrefix(mt, "application/vnd.docker.")
}

// IsManifestType returns true if the media type is an OCI-compatible manifest.
// No support for schema1 manifest.
func IsManifestType(mt string) bool {
	switch mt {
	case MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
		return true
	default:
		return false
	}
}

// IsIndexType returns true if the media type is an OCI-compatible index.
func IsIndexType(mt string) bool {
	switch mt {
	case ocispec.MediaTypeImageIndex, MediaTypeDockerSchema2ManifestList:
		return true
	default:
		return false
	}
}

// IsConfigType returns true if the media type is an OCI-compatible image config.
// No support for containerd checkpoint configs.
func IsConfigType(mt string) bool {
	switch mt {
	case MediaTypeDockerSchema2Config, ocispec.MediaTypeImageConfig:
		return true
	default:
		return false
	}
}

// IsKnownConfig returns true if the media type is a known config type,
// including containerd checkpoint configs
func IsKnownConfig(mt string) bool {
	switch mt {
	case MediaTypeDockerSchema2Config, ocispec.MediaTypeImageConfig,
		MediaTypeContainerd1Checkpoint, MediaTypeContainerd1CheckpointConfig:
		return true
	}
	return false
}

// IsAttestationType returns true if the media type is an attestation type
func IsAttestationType(mt string) bool {
	switch mt {
	case MediaTypeInToto:
		return true
	default:
		return false
	}
}

// ChildGCLabels returns the label for a given descriptor to reference it
func ChildGCLabels(desc ocispec.Descriptor) []string {
	mt := desc.MediaType
	if IsKnownConfig(mt) {
		return []string{"containerd.io/gc.ref.content.config"}
	}

	switch mt {
	case MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest:
		return []string{"containerd.io/gc.ref.content.m."}
	}

	if IsLayerType(mt) {
		return []string{"containerd.io/gc.ref.content.l."}
	}

	return []string{"containerd.io/gc.ref.content."}
}

// ChildGCLabelsFilterLayers returns the labels for a given descriptor to
// reference it, skipping layer media types
func ChildGCLabelsFilterLayers(desc ocispec.Descriptor) []string {
	if IsLayerType(desc.MediaType) {
		return nil
	}
	return ChildGCLabels(desc)
}
