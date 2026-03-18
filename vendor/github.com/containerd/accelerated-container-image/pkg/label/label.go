/*
   Copyright The Accelerated Container Image Authors

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

package label

// support on-demand loading by the labels
const (
	// TargetSnapshotRef is the interface to know that Prepare
	// action is to pull image, not for container Writable snapshot.
	//
	// NOTE: Only available in >= containerd 1.4.0 and containerd.Pull
	// with Unpack option.
	//
	// FIXME(fuweid): With containerd design, we don't know that what purpose
	// snapshotter.Prepare does for. For unpacked image, prepare is for
	// container's rootfs. For pulling image, the prepare is for committed.
	// With label "containerd.io/snapshot.ref" in preparing, snapshotter
	// author will know it is for pulling image. It will be useful.
	//
	// The label is only propagated during pulling image. So, is it possible
	// to propagate by image.Unpack()?
	TargetSnapshotRef = "containerd.io/snapshot.ref"

	// TargetImageRef is the label to mark where the snapshot comes from.
	//
	// TODO(fuweid): Is it possible to use it in upstream?
	TargetImageRef = "containerd.io/snapshot/image-ref"

	// OverlayBDBlobDigest is the annotation key in the manifest to
	// describe the digest of blob in OverlayBD format.
	//
	// NOTE: The annotation is part of image layer blob's descriptor.
	OverlayBDBlobDigest = "containerd.io/snapshot/overlaybd/blob-digest"

	// OverlayBDBlobSize is the annotation key in the manifest to
	// describe the size of blob in OverlayBD format.
	//
	// NOTE: The annotation is part of image layer blob's descriptor.
	OverlayBDBlobSize = "containerd.io/snapshot/overlaybd/blob-size"

	// OverlayBDBlobFsType is the annotation key in the manifest to
	// describe the filesystem type to be mounted as of blob in OverlayBD format.
	//
	// NOTE: The annotation is part of image layer blob's descriptor.
	OverlayBDBlobFsType = "containerd.io/snapshot/overlaybd/blob-fs-type"

	// AccelerationLayer is the annotation key in the manifest to indicate
	// whether a top layer is acceleration layer or not.
	AccelerationLayer = "containerd.io/snapshot/overlaybd/acceleration-layer"

	// RecordTrace tells snapshotter to record trace
	RecordTrace = "containerd.io/snapshot/overlaybd/record-trace"

	// RecordTracePath is the file path to record trace
	RecordTracePath = "containerd.io/snapshot/overlaybd/record-trace-path"

	// ZFileConfig is the config of ZFile
	ZFileConfig = "containerd.io/snapshot/overlaybd/zfile-config"

	// OverlayBD virtual block device size
	OverlayBDVsize = "containerd.io/snapshot/overlaybd/vsize"

	// CRIImageRef is the image-ref from cri
	CRIImageRef = "containerd.io/snapshot/cri.image-ref"

	// TurboOCIDigest is the index annotation key for image layer digest
	FastOCIDigest  = "containerd.io/snapshot/overlaybd/fastoci/target-digest" // legacy
	TurboOCIDigest = "containerd.io/snapshot/overlaybd/turbo-oci/target-digest"

	// TurboOCIMediaType is the index annotation key for image layer media type
	FastOCIMediaType  = "containerd.io/snapshot/overlaybd/fastoci/target-media-type" // legacy
	TurboOCIMediaType = "containerd.io/snapshot/overlaybd/turbo-oci/target-media-type"

	// DownloadRemoteBlob is a label for download remote blob
	DownloadRemoteBlob = "containerd.io/snapshot/overlaybd/download-remote-blob"

	RemoteLabel    = "containerd.io/snapshot/remote"
	RemoteLabelVal = "remote snapshot"

	// OverlayBDVersion is the version number of overlaybd blob
	OverlayBDVersion = "containerd.io/snapshot/overlaybd/version"

	// LayerToTurboOCI is used to convert local layer to turboOCI with tar index
	LayerToTurboOCI = "containerd.io/snapshot/overlaybd/convert2turbo-oci"

	SnapshotType = "containerd.io/snapshot/type"

	// RootfsQuotaLabel sets container rootfs diskquota
	RootfsQuotaLabel = "containerd.io/snapshot/disk_quota"
)

// used in filterAnnotationsForSave (https://github.com/moby/buildkit/blob/v0.11/cache/refs.go#L882)
var OverlayBDAnnotations = []string{
	LocalOverlayBDPath,
	OverlayBDBlobDigest,
	OverlayBDBlobSize,
	OverlayBDBlobFsType,
}

// interface
const (
	// SupportReadWriteMode is used to support writable block device
	// for active snapshotter.
	//
	// By default, multiple active snapshotters can share one block device
	// from parent snapshotter(committed). Like image builder and
	// sandboxed-like container runtime(KataContainer, Firecracker), those
	// cases want to use the block device alone or as writable.
	// There are two ways to provide writable devices:
	//  - 'dir' mark the snapshotter
	//    as wriable block device and mount it on rootfs.
	//  - 'dev' mark the snapshotter
	//    as wriable block device without mount.
	SupportReadWriteMode = "containerd.io/snapshot/overlaybd.writable"

	// LocalOverlayBDPath is used to export the commit file path.
	//
	// NOTE: Only used in image build.
	LocalOverlayBDPath = "containerd.io/snapshot/overlaybd.localcommitpath"
)
