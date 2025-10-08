package main

import (
	"context"
	"net"
	"os"
	"path/filepath"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

type driverServer struct{}

// Required by RegisterIdentityServer
func (d *driverServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          "csi",
		VendorVersion: "0.1.0",
	}, nil
}
func (d *driverServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	return &csi.GetPluginCapabilitiesResponse{}, nil
}
func (d *driverServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}

// Required by RegisterControllerServer
func (d *driverServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	return &csi.CreateVolumeResponse{}, nil

}
func (d *driverServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	return &csi.DeleteVolumeResponse{}, nil
}
func (d *driverServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return &csi.ControllerPublishVolumeResponse{}, nil
}
func (d *driverServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}
func (d *driverServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return &csi.ValidateVolumeCapabilitiesResponse{}, nil
}
func (d *driverServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return &csi.ListVolumesResponse{}, nil
}
func (d *driverServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return &csi.GetCapacityResponse{}, nil
}
func (d *driverServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{}, nil
}
func (d *driverServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return &csi.CreateSnapshotResponse{}, nil
}
func (d *driverServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return &csi.DeleteSnapshotResponse{}, nil
}
func (d *driverServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return &csi.ListSnapshotsResponse{}, nil
}
func (d *driverServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return &csi.ControllerExpandVolumeResponse{}, nil
}
func (d *driverServer) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return &csi.ControllerGetVolumeResponse{}, nil
}

// Required by RegisterNodeServer
func (d *driverServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{},
	}, nil
}
func (d *driverServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{}, nil
}
func (d *driverServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return &csi.NodeGetVolumeStatsResponse{}, nil
}
func (d *driverServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, nil
}
func (d *driverServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if err := os.MkdirAll(req.TargetPath, 0o755); err != nil {
		return nil, err
	}

	// Create test file so it's content can be verified from container
	file, err := os.Create(filepath.Join(req.TargetPath, "data.txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	_, err = file.WriteString("this is a csi mounted file")
	if err != nil {
		return nil, err
	}

	return &csi.NodePublishVolumeResponse{}, nil
}
func (d *driverServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	return &csi.NodeUnpublishVolumeResponse{}, nil
}
func (d *driverServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return &csi.NodeStageVolumeResponse{}, nil
}
func (d *driverServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func main() {
	p, err := filepath.Abs(filepath.Join("run", "docker", "plugins"))
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		panic(err)
	}
	l, err := net.Listen("unix", filepath.Join(p, "csi.sock"))
	if err != nil {
		panic(err)
	}

	server := grpc.NewServer()
	ds := &driverServer{}
	csi.RegisterIdentityServer(server, ds)
	csi.RegisterControllerServer(server, ds)
	csi.RegisterNodeServer(server, ds)
	server.Serve(l)
}
