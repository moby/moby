package resourceapi

import (
	"errors"
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var (
	errInvalidArgument = errors.New("invalid argument")
)

// ResourceAllocator handles resource allocation of cluster entities.
type ResourceAllocator struct {
	store *store.MemoryStore
}

// New returns an instance of the allocator
func New(store *store.MemoryStore) *ResourceAllocator {
	return &ResourceAllocator{store: store}
}

// AttachNetwork allows the node to request the resources
// allocation needed for a network attachment on the specific node.
// - Returns `InvalidArgument` if the Spec is malformed.
// - Returns `NotFound` if the Network is not found.
// - Returns `PermissionDenied` if the Network is not manually attachable.
// - Returns an error if the creation fails.
func (ra *ResourceAllocator) AttachNetwork(ctx context.Context, request *api.AttachNetworkRequest) (*api.AttachNetworkResponse, error) {
	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return nil, err
	}

	var network *api.Network
	ra.store.View(func(tx store.ReadTx) {
		network = store.GetNetwork(tx, request.Config.Target)
		if network == nil {
			if networks, err := store.FindNetworks(tx, store.ByName(request.Config.Target)); err == nil && len(networks) == 1 {
				network = networks[0]
			}
		}
	})
	if network == nil {
		return nil, grpc.Errorf(codes.NotFound, "network %s not found", request.Config.Target)
	}

	if !network.Spec.Attachable {
		return nil, grpc.Errorf(codes.PermissionDenied, "network %s not manually attachable", request.Config.Target)
	}

	t := &api.Task{
		ID:     identity.NewID(),
		NodeID: nodeInfo.NodeID,
		Spec: api.TaskSpec{
			Runtime: &api.TaskSpec_Attachment{
				Attachment: &api.NetworkAttachmentSpec{
					ContainerID: request.ContainerID,
				},
			},
			Networks: []*api.NetworkAttachmentConfig{
				{
					Target:    network.ID,
					Addresses: request.Config.Addresses,
				},
			},
		},
		Status: api.TaskStatus{
			State:     api.TaskStateNew,
			Timestamp: ptypes.MustTimestampProto(time.Now()),
			Message:   "created",
		},
		DesiredState: api.TaskStateRunning,
		// TODO: Add Network attachment.
	}

	if err := ra.store.Update(func(tx store.Tx) error {
		return store.CreateTask(tx, t)
	}); err != nil {
		return nil, err
	}

	return &api.AttachNetworkResponse{AttachmentID: t.ID}, nil
}

// DetachNetwork allows the node to request the release of
// the resources associated to the network attachment.
// - Returns `InvalidArgument` if attachment ID is not provided.
// - Returns `NotFound` if the attachment is not found.
// - Returns an error if the deletion fails.
func (ra *ResourceAllocator) DetachNetwork(ctx context.Context, request *api.DetachNetworkRequest) (*api.DetachNetworkResponse, error) {
	if request.AttachmentID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return nil, err
	}

	if err := ra.store.Update(func(tx store.Tx) error {
		t := store.GetTask(tx, request.AttachmentID)
		if t == nil {
			return grpc.Errorf(codes.NotFound, "attachment %s not found", request.AttachmentID)
		}
		if t.NodeID != nodeInfo.NodeID {
			return grpc.Errorf(codes.PermissionDenied, "attachment %s doesn't belong to this node", request.AttachmentID)
		}

		return store.DeleteTask(tx, request.AttachmentID)
	}); err != nil {
		return nil, err
	}

	return &api.DetachNetworkResponse{}, nil
}
