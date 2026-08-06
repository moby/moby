package capability

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/moby/swarmkit/v2/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func CheckArguments(req *api.VolumeAssignment) error {
	if len(req.VolumeId) == 0 {
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
	case api.VolumeAccessMode_SINGLE_NODE:
		switch am.Sharing {
		case api.VolumeAccessMode_NONE, api.VolumeAccessMode_ONE_WRITER, api.VolumeAccessMode_ALL:
			mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
		case api.VolumeAccessMode_READ_ONLY:
			mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY
		}
	case api.VolumeAccessMode_MULTI_NODE:
		switch am.Sharing {
		case api.VolumeAccessMode_READ_ONLY:
			mode = csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY
		case api.VolumeAccessMode_ONE_WRITER:
			mode = csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER
		case api.VolumeAccessMode_ALL:
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
