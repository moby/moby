/*
 * Copyright (c) 2022. Nydus Developers. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package converter

const (
	ManifestOSFeatureNydus   = "nydus.remoteimage.v1"
	ManifestConfigNydus      = "application/vnd.nydus.image.config.v1+json"
	MediaTypeNydusBlob       = "application/vnd.oci.image.layer.nydus.blob.v1"
	BootstrapFileNameInLayer = "image/image.boot"

	ManifestNydusCache = "containerd.io/snapshot/nydus-cache"

	LayerAnnotationFSVersion          = "containerd.io/snapshot/nydus-fs-version"
	LayerAnnotationNydusBlob          = "containerd.io/snapshot/nydus-blob"
	LayerAnnotationNydusBlobDigest    = "containerd.io/snapshot/nydus-blob-digest"
	LayerAnnotationNydusBlobSize      = "containerd.io/snapshot/nydus-blob-size"
	LayerAnnotationNydusBootstrap     = "containerd.io/snapshot/nydus-bootstrap"
	LayerAnnotationNydusSourceChainID = "containerd.io/snapshot/nydus-source-chainid"
	LayerAnnotationNydusEncryptedBlob = "containerd.io/snapshot/nydus-encrypted-blob"
	LayerAnnotationNydusSourceDigest  = "containerd.io/snapshot/nydus-source-digest"
	LayerAnnotationNydusTargetDigest  = "containerd.io/snapshot/nydus-target-digest"

	LayerAnnotationNydusReferenceBlobIDs = "containerd.io/snapshot/nydus-reference-blob-ids"

	LayerAnnotationUncompressed = "containerd.io/uncompressed"
)
