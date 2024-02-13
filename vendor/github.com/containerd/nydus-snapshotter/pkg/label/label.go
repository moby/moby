/*
 * Copyright (c) 2020. Ant Group. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package label

import (
	snpkg "github.com/containerd/containerd/pkg/snapshotters"
)

// For package compatibility, we still keep the old exported name here.
var AppendLabelsHandlerWrapper = snpkg.AppendInfoHandlerWrapper

// For package compatibility, we still keep the old exported name here.
const (
	CRIImageRef       = snpkg.TargetRefLabel
	CRIImageLayers    = snpkg.TargetImageLayersLabel
	CRILayerDigest    = snpkg.TargetLayerDigestLabel
	CRIManifestDigest = snpkg.TargetManifestDigestLabel
)

const (
	// Marker for remote snapshotter to handle the pull request.
	// During image pull, the containerd client calls Prepare API with the label containerd.io/snapshot.ref.
	// This is a containerd-defined label which contains ChainID that targets a committed snapshot that the
	// client is trying to prepare.
	TargetSnapshotRef = "containerd.io/snapshot.ref"

	// A bool flag to mark the blob as a Nydus data blob, set by image builders.
	NydusDataLayer = "containerd.io/snapshot/nydus-blob"
	// A bool flag to mark the blob as a nydus bootstrap, set by image builders.
	NydusMetaLayer = "containerd.io/snapshot/nydus-bootstrap"
	// The referenced blob sha256 in format of `sha256:xxx`, set by image builders.
	NydusRefLayer = "containerd.io/snapshot/nydus-ref"
	// The blobID of associated layer, also marking the layer as a nydus tarfs, set by the snapshotter
	NydusTarfsLayer = "containerd.io/snapshot/nydus-tarfs"
	// Dm-verity information for image block device
	NydusImageBlockInfo = "containerd.io/snapshot/nydus-image-block"
	// Dm-verity information for layer block device
	NydusLayerBlockInfo = "containerd.io/snapshot/nydus-layer-block"
	// Annotation containing secret to pull images from registry, set by the snapshotter.
	NydusImagePullSecret = "containerd.io/snapshot/pullsecret"
	// Annotation containing username to pull images from registry, set by the snapshotter.
	NydusImagePullUsername = "containerd.io/snapshot/pullusername"
	// Proxy image pull actions to other agents.
	NydusProxyMode = "containerd.io/snapshot/nydus-proxy-mode"
	// A bool flag to enable integrity verification of meta data blob
	NydusSignature = "containerd.io/snapshot/nydus-signature"

	// A bool flag to mark the blob as a estargz data blob, set by the snapshotter.
	StargzLayer = "containerd.io/snapshot/stargz"

	// volatileOpt is a key of an optional label to each snapshot.
	// If this optional label of a snapshot is specified, when mounted to rootdir
	// this snapshot will include volatile option
	OverlayfsVolatileOpt = "containerd.io/snapshot/overlay.volatile"

	// A bool flag to mark it is recommended to run this image with tarfs mode, set by image builders.
	// runtime can decide whether to rely on this annotation
	TarfsHint = "containerd.io/snapshot/tarfs-hint"
)

func IsNydusDataLayer(labels map[string]string) bool {
	_, ok := labels[NydusDataLayer]
	return ok
}

func IsNydusMetaLayer(labels map[string]string) bool {
	_, ok := labels[NydusMetaLayer]
	return ok
}

func IsTarfsDataLayer(labels map[string]string) bool {
	_, ok := labels[NydusTarfsLayer]
	return ok
}

func IsNydusProxyMode(labels map[string]string) bool {
	_, ok := labels[NydusProxyMode]
	return ok
}

func HasTarfsHint(labels map[string]string) bool {
	_, ok := labels[TarfsHint]
	return ok
}
