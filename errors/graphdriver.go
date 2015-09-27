package errors

// This file contains all of the errors that can be generated from the
// docker/daemon/graphdriver component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeAufsNotSupported is generated when host does not suppport aufs filesystem.
	ErrorCodeAufsNotSupported = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "AUFSNOTSUPPORTED",
		Message:        "AUFS was not found in /proc/filesystems",
		Description:    "Indicates that the host does not support aufs filesystem",
		HTTPStatusCode: http.StatusInternalServerError,
	})
	// ErrorCodeAufsMountFailed is generated
	ErrorCodeAufsMountFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "AUFSMOUNTFAILED",
		Message:        "error creating aufs mount to %s: %v",
		Description:    "Failed to mount the specified ID on aufs filesystem",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeAufsRelocFailed is generated when trying to relocate or simply creating a symlink to a newpath.
	ErrorCodeAufsRelocFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "AUFSRELOCFAILED",
		Message:        "Unable to relocate %s to %s: Rename err %s Symlink err %s",
		Description:    "Indicates that the oldpath could not be relocated to a newpath or a symlink creation failed",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBtrfsOpenDirFailed is generated when it fails to open a btrfs directory
	ErrorCodeBtrfsOpenDirFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BTRFSOPENDIRFAILED",
		Message:        "Can't open dir",
		Description:    "Indicates an error occured when trying to open a Btrfs directory.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBtrfsSubVolCreateFailed is generated when it fails to create a btrfs subvolume
	ErrorCodeBtrfsSubVolCreateFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BTRFSSUBVOLCREATEFAILED",
		Message:        "Failed to create btrfs subvolume: %v",
		Description:    "Indicates an error occured when trying to open a Btrfs directory.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBtrfsSnapshotCreateFailed is generated when it fails to create a btrfs snapshot
	ErrorCodeBtrfsSnapshotCreateFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BTRFSSNAPSHOTCREATEFAILED",
		Message:        "Failed to create btrfs snapshot: %v",
		Description:    "Indicates an error occured when breating a Btrfs snapshot.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBtrfsErrIsSubVol is generated when Btrfs fails to test a given path is a subvolume
	ErrorCodeBtrfsErrIsSubVol = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BTRFSERRISSUBVOL",
		Message:        "Failed to test if %s is a btrfs subvolume",
		Description:    "Failed to test is a given path is a btrfs subvolume",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBtrfsErrDelSubVol is generated when error occured deleting a subvolume in Btrfs.
	ErrorCodeBtrfsErrDelSubVol = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BTRFSERRDELSUBVOL",
		Message:        "Failed to destroy btrfs child subvolume (%s) of parent (%s): %v",
		Description:    "Indicates that a failure occurred deleting a Btrfs subvolume.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBtrfsErrWalkSubVols is generated when error occurs walking through all subvolumes in a Btrfs directory.
	ErrorCodeBtrfsErrWalkSubVols = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BTRFSERRWALKSUBVOLS",
		Message:        "Recursively walking subvolumes for %s failed: %v",
		Description:    "Indicates an error when walking through all subvolumes to delete",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBtrfsErrDelSnapshot is generated when deleting a snapshot as part of delete subvolume in Btrfs.
	ErrorCodeBtrfsErrDelSnapshot = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BTRFSERRDELSNAPSHOT",
		Message:        "Failed to destroy btrfs snapshot %s for %s: %v",
		Description:    "Indicates an error occurred when deleting a snapshot in Btrfs",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBtrfsNotADir is generated if the given string does not represent a Btrfs directory.
	ErrorCodeBtrfsNotADir = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BTRFSNOTADIR",
		Message:        "%s: not a directory",
		Description:    "Indicates the given string does not represent a Btrfs directory.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrLoopBack is generated when it files to grow a loopback file.
	ErrorCodeDMapErrLoopBack = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRLOOPBACK",
		Message:        "Unable to grow loopback file %s: %v",
		Description:    "Indicates that the Device mapper is not able to grow the loopback file",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrSetTxID is generated when device mapper fails to set the transaction id.
	ErrorCodeDMapErrSetTxID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRSETTXID",
		Message:        "Error setting devmapper transaction ID: %s",
		Description:    "Indicates an error occurred when device mapper fails to set transaction ID.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrRemoveMetadata is generated when device mapper fails to remove the metadata file.
	ErrorCodeDMapErrRemoveMetadata = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRREMOVEMETADATA",
		Message:        "Error removing metadata file %s: %s",
		Description:    "Indicate that an error occurred while device mapper trying to remove the metadata file.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrCreateMetadata is generated  when device mapper fails to create the metadata file.
	ErrorCodeDMapErrCreateMetadata = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRCREATEMETADATA",
		Message:        "Error creating metadata file: %s",
		Description:    "Indicate that an error occurred while device mapper trying to create the metadata file.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrWriteMetadata is generated when device mapper fails to write to the metadata file.
	ErrorCodeDMapErrWriteMetadata = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRWRITEMETADATA",
		Message:        "Error writing metadata to %s: %s",
		Description:    "Indicate that an error occurred while device mapper trying to write to the metadata file.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrSyncMetadata is generated when device mapper fails to sync the metadata file.
	ErrorCodeDMapErrSyncMetadata = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRSYNCMETADATA",
		Message:        "Error syncing metadata file %s: %s",
		Description:    "Indicate that an error occurred while device mapper trying to sync the metadata file.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrCloseMetadata is generated when device mapper fails to close the metadata file.
	ErrorCodeDMapErrCloseMetadata = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRCLOSEMETADATA",
		Message:        "Error closing metadata file %s: %s",
		Description:    "Indicate that an error occurred while device mapper trying to close the metadata file.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrCommitMetadata is generated when device mapper fails to commit to the metadata file.
	ErrorCodeDMapErrCommitMetadata = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRCOMMITMETADATA",
		Message:        "Error committing metadata file %s: %s",
		Description:    "Indicate that an error occurred while device mapper trying to commit to the metadata file.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrLoadMetadata is generated when device mapper fails to load the metadata file.
	ErrorCodeDMapErrLoadMetadata = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRLOADMETADATA",
		Message:        "Error loading device metadata file %s",
		Description:    "Indicate that an error occurred while device mapper trying to load the metadata file.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrEncodeMetadata is generated when encoding metdata into json format fails.
	ErrorCodeDMapErrEncodeMetadata = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRENCODEMETADATA",
		Message:        "Error encoding metadata to json: %s",
		Description:    "Indicates that devimce mapper fails to encode metadata into json format",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapUnknownDevice is generated when the device id is not know to the device mapper.
	ErrorCodeDMapUnknownDevice = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPUNKNOWNDEVICE",
		Message:        "Unknown device %s",
		Description:    "Indicates that the device is not known to device mapper.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrDefRemCancel is generated when error occurred during cancellation of deferred removal to activate a device for device mapper.
	ErrorCodeDMapErrDefRemCancel = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRDEFREMCANCEL",
		Message:        "Deivce Deferred Removal Cancellation Failed: %s",
		Description:    "Indicate and error cancelling a deferred removal in device mapper.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrDetectFSType is generated when device mapper fails to detect a filesystem type.
	ErrorCodeDMapErrDetectFSType = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRDETECTFSTYPE",
		Message:        "unable to detect filesystem type of %s, short read",
		Description:    "Indicates that the device mapper fails to detect a filesystem type.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapUnsupportedFSType is generated when device mapper detects a unsupported filesystem.
	ErrorCodeDMapUnsupportedFSType = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPUNSUPPORTEDFSTYPE",
		Message:        "Unsupported filesystem type %s",
		Description:    "Indicates the filesystem on the device is not supported by device mapper.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapUnknownFSType is generated when device mapper detects a unknown filesystem.
	ErrorCodeDMapUnknownFSType = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPUNKNOWNFSTYPE",
		Message:        "Unknown filesystem type on %s",
		Description:    "Indicates the filesystem is unknown to device mapper.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapUnsupportedFS is generated when device mapper detects a unsupported filesystem.
	ErrorCodeDMapUnsupportedFS = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPUNSUPPORTEDFS",
		Message:        "Unsupported filesystem %s\n",
		Description:    "Indicates the filesystem on the device is not supported by device mapper.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapNoDeviceID is generated when device mapper is unable to find a free device id.
	ErrorCodeDMapNoDeviceID = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPNODEVICEID",
		Message:        "Unable to find a free device ID",
		Description:    "Indicates and device mapper fails to find a free device id",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapVerifyUUIDFailed is generated when device mapper fails to compare base device UUID with the stored device UUID.
	ErrorCodeDMapVerifyUUIDFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPVERIFYUUIDFAILED",
		Message:        "Current Base Device UUID:%s does not match with stored UUID:%s",
		Description:    "Indicates that the device mapper stored device UUID does not match with the device UUID",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapQuerySaveUUIDFailed is generated when device mapper fails to query and save the base device UUID
	ErrorCodeDMapQuerySaveUUIDFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPQUERYSAVEUUIDFAILED",
		Message:        "Could not query and save base device UUID:%v",
		Description:    "Indicates a failure occurred when query old device UUID and save a base device UUID.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapVerifyBaseDevFailed is generated when base device UUID verification failed.
	ErrorCodeDMapVerifyBaseDevFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMPAVERIFYBASEDEVFAILED",
		Message:        "Base Device UUID verification failed. Possibly using a different thin pool than last invocation:%v",
		Description:    "Indicate device mapper failed to verify the base device UUID.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrPoolUsed is generated when device mapper detects that thin pool status shows that it already has used data.
	ErrorCodeDMapErrPoolUsed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRPOOLUSED",
		Message:        "Unable to take ownership of thin-pool (%s) that already has used data blocks",
		Description:    "Indicates that thin pool status shows that it already has used data",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrPoolTxUsed is generated device mapper detects a existing trasaction id in the pool.
	ErrorCodeDMapErrPoolTxUsed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRPOOLTXUSED",
		Message:        "Unable to take ownership of thin-pool (%s) with non-zero transaction ID",
		Description:    "Indicates that device mapper detects a existing trasaction id in the pool.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrShrinkDataFile is generated when device mapper is unable to shrink the data file.
	ErrorCodeDMapErrShrinkDataFile = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRSHRINKDATAFILE",
		Message:        "Can't shrink file",
		Description:    "Indicates that device mapper is unable to shrink the data file",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapNoDataLoopback is generated device mapper is not able to find the loopback device for data
	ErrorCodeDMapNoDataLoopback = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPNODATALOOPBACK",
		Message:        "Unable to find loopback mount for: %s",
		Description:    "Indicates that device mapper is not able to find the loopback device for data",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapNoMetadataLoopback is generated device mapper is not able to find the loopback device for metadata
	ErrorCodeDMapNoMetadataLoopback = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPNOMETADATALOOPBACK",
		Message:        "Unable to find loopback mount for: %s",
		Description:    "Indicates that device mapper is not able to find the loopback device for metadata",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrGrowLoopback is generated when device mapper fails to grow the loopback file.
	ErrorCodeDMapErrGrowLoopback = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRGROWLOOPBACK",
		Message:        "Unable to grow loopback file: %s",
		Description:    "Indicates that device mapper fails to grow the loopback file.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrUpdateLoopback is generated when device mapper fails to update the loopback capacity.
	ErrorCodeDMapErrUpdateLoopback = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRUPDATELOOPBACK",
		Message:        "Unable to grow loopback file: %s",
		Description:    "Indicates that device mapper fails to update the loopback capacity.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrSuspendPool is generated when device mapper is unable to suspend the pool
	ErrorCodeDMapErrSuspendPool = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRSUSPENDPOOL",
		Message:        "Unable to suspend pool: %s",
		Description:    "Indicates that device mapper is unable to suspend the pool.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrReloadPool is generated when device mapper is unable to reload the pool
	ErrorCodeDMapErrReloadPool = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRRELOADPOOL",
		Message:        "Unable to reload pool: %s",
		Description:    "Indicates that device mapper is unable to reload the pool.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrResumePool is generated when device mapper is unable to resume the pool
	ErrorCodeDMapErrResumePool = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRRESUMEPOOL",
		Message:        "Unable to resume pool: %s",
		Description:    "Indicates that device mapper is unable to resume the pool.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrRollbackTx is generated when device mapper fails to rollback a transaction.
	ErrorCodeDMapErrRollbackTx = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPROLLBACKTX",
		Message:        "Rolling back open transaction failed: %s",
		Description:    "Indicates that device mapper fails to rollback a transaction.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrSaveTx is generated when device mapper fails to save the trasaction metadata.
	ErrorCodeDMapErrSaveTx = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRSAVETX",
		Message:        "Error saving transaction metadata: %s",
		Description:    "Indicates that device mapper fails to save the trasaction metadata.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrNoLoopback is generated when loopback device is not found for the file.
	ErrorCodeDMapErrNoLoopback = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRNOLOOPBACK",
		Message:        "[devmapper]: Unable to find loopback mount for: %s",
		Description:    "Indicates the device mapper is unable to fine the loopback device for the file.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapNoDir is generated when the directory is not found in the root filesystem.
	ErrorCodeDMapNoDir = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRNODIR",
		Message:        "Error looking up dir %s: %s",
		Description:    "Indicate the directory specified is not found within the scope of the root.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDmapErrDevExists is generated when the device already added to the deviceset.
	ErrorCodeDmapErrDevExists = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRDEVEXISTS",
		Message:        "device %s already exists",
		Description:    "Indicates that device mapper detects that the device is already added to the deviceset.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrDevMounted is generated when device mapper detects that the device is already mounted.
	ErrorCodeDMapErrDevMounted = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRDEVMOUNTED",
		Message:        "Trying to mount devmapper device in multiple places (%s, %s)",
		Description:    "Indicates that the device mapper detects that the device is already mounted",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrDevActivate is generated when error occurred activating a device.
	ErrorCodeDMapErrDevActivate = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRDEVACTIVATE",
		Message:        "Error activating devmapper device for '%s': %s",
		Description:    "Indicates that device mapper failed to activate a device.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDmapErrDevMount is generated when unable to mount a device to specific path.
	ErrorCodeDmapErrDevMount = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRDEVMOUNT",
		Message:        "Error mounting '%s' on '%s': %s",
		Description:    "Indicates that device mapper is unable to mount the device at the specified path.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapErrDevUnmount is generated when device mapper detects that the device is not mounted.
	ErrorCodeDMapErrDevUnmount = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPERRDEVUNMOUNT",
		Message:        "UnmountDevice: device not-mounted id %s",
		Description:    "Indicate that unmount caused an error because device mapper detects that the device is not mounted.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDMapUnknownOption is generated when device mapper detects unknown option when creating a deviceset.
	ErrorCodeDMapUnknownOption = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DMAPUNKNOWNOPTION",
		Message:        "Unknown option %s\n",
		Description:    "Indicate that device mapper detects unknown option when creating a deviceset",
		HTTPStatusCode: http.StatusInternalServerError,
	})
	// ErrorCodeOVApplyDiffFallback is generated to indicate normal ApplyDiff is applied as a fallback from Naive diff writer.
	ErrorCodeOVApplyDiffFallback = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "OVAPPLYDIFFFALLBACK",
		Message:        "Fall back to normal ApplyDiff",
		Description:    "Indicates normal ApplyDiff is applied as a fallback from Naive diff writer.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeOVErrCreateMount is generated when overlay driver fails to create a mount for a given filesystem
	ErrorCodeOVErrCreateMount = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "OVERRCREATEMOUNT",
		Message:        "error creating overlay mount to %s: %v",
		Description:    "Indicates ovaerlay failed to create a mount for a given filesystem",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVFSErrDetectParent is generated where VFS cannot create filesystem under a specified parent.
	ErrorCodeVFSErrDetectParent = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VFSERRDETECTPARENT",
		Message:        "%s: %s",
		Description:    "Indicates that VFS unable to detect the parent of the given filesystem",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeVFSErrFindDir is generated when the specified id is not associated with a valid directory.
	ErrorCodeVFSErrFindDir = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "VFSERRFINDDIR",
		Message:        "%s: not a directory",
		Description:    "Indicated that the CFS is unable to find a valid directory for the given id",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrRWLayer is generated read/write layer is created without a parent.
	ErrorCodeWinErrRWLayer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRRWLAYER",
		Message:        "Cannot create a read/write layer without a parent layer.",
		Description:    "Indicates Windows detected the creation of read/write layer without a parent.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrNoParent is generated when the parent layer is missing from the filesystem.
	ErrorCodeWinErrNoParent = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRNOPARENT",
		Message:        "Cannot create layer with missing parent %s: %s",
		Description:    "Indicates that the parent filesystem is missing",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinUnsupportedChanges is generated to indicate Changes is not implemented between layers on Windows.
	ErrorCodeWinUnsupportedChanges = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINUNSUPPORTEDCHANGES",
		Message:        "The Windows graphdriver does not support Changes()",
		Description:    "Indicates Windows does not implement changes api to show changes between layers.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrRestoreBaseImage is generated Windows fails to restore base image.
	ErrorCodeWinErrRestoreBaseImage = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRRESTOREBASEIMAGE",
		Message:        "Failed to restore base images: %s",
		Description:    "Indicates the a failure has occurred while restoring base image on Windows.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrJSONUnmarshal is generated when Windows fails to unmarshal imagedate in to JSON.
	ErrorCodeWinErrJSONUnmarshal = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRJSONUNMARSHAL",
		Message:        "JSON unmarshal returned error=%s",
		Description:    "Indicates that Windows fails to unmarshal imagedate in to JSON.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrReadLayers is generated when Windows fails to read the layer chain file.
	ErrorCodeWinErrReadLayers = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRREADLAYERS",
		Message:        "Unable to read layerchain file - %s",
		Description:    "Indicates an erro occurred on Windows when reading a layer chain file",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrUnmarshalLayers is generated when Windows fails to unmarshal layerchaing in to json.
	ErrorCodeWinErrUnmarshalLayers = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRUNMARSHALLLAYERS",
		Message:        "Failed to unmarshall layerchain json - %s",
		Description:    "Indicates that Windows fails to unmarshal layerchaing in to json.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrMarshalLayers is generated when Windows fails to marshal layerchaing in to json.
	ErrorCodeWinErrMarshalLayers = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRMARSHALLLAYERS",
		Message:        "Failed to marshall layerchain json - %s",
		Description:    "Indicates that Windows fails to marshal layerchaing in to json.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWinErrWriteLayerFile is generated when Windows fail to write to layerchain file.
	ErrorCodeWinErrWriteLayerFile = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WINERRWRITELAYERFILE",
		Message:        "Unable to write layerchain file - %s",
		Description:    "Indicates that Windows fail to write to layerchain file.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeZFSErrNoRoot is generated if the ZFS init cannot find the root of the filesystem.
	ErrorCodeZFSErrNoRoot = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ZFSERRNOROOT",
		Message:        "Cannot find root filesystem %s: %v",
		Description:    "Indicates that ZFS init cannot find the root of the filesystem.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeZFSErrMissingRootDS is generated when ZFS cannot find the specified root dataset in the filesystem list.
	ErrorCodeZFSErrMissingRootDS = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ZFSERRMISSINGROOTDS",
		Message:        "BUG: zfs get all -t filesystem -rHp '%s' should contain '%s'",
		Description:    "Indicates that ZFS init cannot find the specified root dataset in the filesystem list.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeZFSUnknownOption is generated when ZFS detects unknown option when creating a deviceset.
	ErrorCodeZFSUnknownOption = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ZFSUNKNOWNOPTION",
		Message:        "Unknown option %s\n",
		Description:    "Indicate that ZFS detects unknown option when creating a deviceset",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeZFSErrAccess is generated when ZFS is unable access the root directory of the dataset.
	ErrorCodeZFSErrAccess = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ZFSERRACCESS",
		Message:        "Failed to access '%s': %",
		Description:    "Indicates that ZFS is unable access the root directory of the dataset.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeZFSErrNoDS is generated when ZFS cannot find the dataset mounted.
	ErrorCodeZFSErrNoDS = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ZFSERRNODS",
		Message:        "Failed to find zfs dataset mounted on '%s' in /proc/mount",
		Description:    "Indicates that ZFS is unable to find the dataset mounted.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeZFSErrMount is generated when ZFS fails to mount the filesytem for given id.
	ErrorCodeZFSErrMount = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ZFSERRMOUNT",
		Message:        "error creating zfs mount of %s to %s: %v",
		Description:    "Indicates that ZFS fails to mount the filesytem for given id.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeZFSErrUnmount is generated when ZFS fails to unmount the filesytem for given id.
	ErrorCodeZFSErrUnmount = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "ZFSERRUNMOUNT",
		Message:        "error unmounting to %s: %v",
		Description:    "Indicates that ZFS fails to mount the filesytem for given id.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGDNotSupported is generated created when the graph driver is not supported.
	ErrorCodeGDNotSupported = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GDNOTSUPPORTED",
		Message:        "driver not supported",
		Description:    "Indicates that the graph driver is not supported.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGDErrPrereqs is generated when the graph driver does not meet the pre-requisites.
	ErrorCodeGDErrPrereqs = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GCERRPREREQS",
		Message:        "prerequisites for driver not satisfied (wrong filesystem?)",
		Description:    "Indicates that the graph driver has not met the pre-requisites",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGDErrFSNotSupported is generated when the graph driver detects unsupported filesystem.
	ErrorCodeGDErrFSNotSupported = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GDERRFSNOTSUPPORTED",
		Message:        "backing file system is unsupported for this graph driver",
		Description:    "Indicates that the graph driver detects unsupported filesystem.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGDErrRegister is generated when registering a init fuction for the graph driver.
	ErrorCodeGDErrRegister = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GDERRREGISTER",
		Message:        "Name already registered %s",
		Description:    "Indicates that the init fuction for the graph driver is already registered",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGDNoStorage is generated when the graph driver fails to find a supported backend storage.
	ErrorCodeGDNoStorage = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GDNOSTORAGE",
		Message:        "No supported storage backend found",
		Description:    "Indicates that the graph driver fails to find a supported backend storage.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGDErrMultipleDrivers is generated when graph driver detects existence of multiple graph drivers used before.
	ErrorCodeGDErrMultipleDrivers = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GDERRMULTIPLEDRIVERS",
		Message:        "%q contains other graphdrivers: %s; Please cleanup or explicitly choose storage driver (-s <DRIVER>)",
		Description:    "Indicates that graph driver detects existence of multiple graph drivers used before.",
		HTTPStatusCode: http.StatusInternalServerError,
	})
)
