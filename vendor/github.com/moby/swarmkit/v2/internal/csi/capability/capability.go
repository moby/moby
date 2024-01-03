package capability

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/moby/swarmkit/v2/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func CheckArguments(req *api.VolumeAssignment) error {
	if len(req.VolumeID) == 0 {
		return status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if req.AccessMode == nil {
		return status.Error(codes.InvalidArgument, "AccessMode missing in request")
	}
	return nil
}

func MakeCapability(am *api.VolumeAccessMode) *csi.VolumeCapability {
	var mode csi.VolumeCapability_AccessMode_Mode
	switch am.Scope {
	case api.VolumeScopeSingleNode:
		switch am.Sharing {
		case api.VolumeSharingNone, api.VolumeSharingOneWriter, api.VolumeSharingAll:
			mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
		case api.VolumeSharingReadOnly:
			mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY
		}
	case api.VolumeScopeMultiNode:
		switch am.Sharing {
		case api.VolumeSharingReadOnly:
			mode = csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY
		case api.VolumeSharingOneWriter:
			mode = csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER
		case api.VolumeSharingAll:
			mode = csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER
		}
	}

	capability := &csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: mode,
		},
	}

	if block := am.GetBlock(); block != nil {
		capability.AccessType = &csi.VolumeCapability_Block{
			// Block type is empty.
			Block: &csi.VolumeCapability_BlockVolume{},
		}
	}

	if mount := am.GetMount(); mount != nil {
		capability.AccessType = &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{
				FsType:     mount.FsType,
				MountFlags: mount.MountFlags,
			},
		}
	}

	return capability
}
