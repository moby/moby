package main

import (
	"context"
	"net"
	"os"
	"path/filepath"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

type driverServer struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedControllerServer
	csi.UnimplementedNodeServer
}

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
func (d *driverServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	return &csi.CreateVolumeResponse{}, nil
}
func (d *driverServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	return &csi.DeleteVolumeResponse{}, nil
}
func (d *driverServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	return &csi.NodePublishVolumeResponse{}, nil
}
func (d *driverServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	return &csi.NodeUnpublishVolumeResponse{}, nil
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
