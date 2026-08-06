package resourceapi

import (
	"context"
	"errors"
	"time"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/ca"
	"github.com/moby/swarmkit/v2/identity"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"github.com/moby/swarmkit/v2/protobuf/ptypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	errInvalidArgument = errors.New("invalid argument")
)

// ResourceAllocator handles resource allocation of cluster entities.
type ResourceAllocator struct {
	api.UnimplementedResourceAllocatorServer

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
	if request.Config == nil {
		return nil, status.Error(codes.InvalidArgument, errInvalidArgument.Error())
	}

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
		return nil, status.Errorf(codes.NotFound, "network %s not found", request.Config.Target)
	}

	if !network.Spec.GetAttachable() {
		return nil, status.Errorf(codes.PermissionDenied, "network %s not manually attachable", request.Config.Target)
	}

	t := &api.Task{
		Id:     identity.NewID(),
		NodeId: nodeInfo.NodeID,
		// Annotations and ServiceAnnotations were non-nullable before the
		// migration to the standard protobuf runtime; keep them always
		// present so API consumers can rely on the old object invariant.
		Annotations:        &api.Annotations{},
		ServiceAnnotations: &api.Annotations{},
		Spec: &api.TaskSpec{
			Runtime: &api.TaskSpec_Attachment{
				Attachment: &api.NetworkAttachmentSpec{
					ContainerId: request.ContainerId,
				},
			},
			Networks: []*api.NetworkAttachmentConfig{
				{
					Target:    network.Id,
					Addresses: request.Config.Addresses,
				},
			},
		},
		Status: &api.TaskStatus{
			State:     api.TaskState_NEW,
			Timestamp: ptypes.MustTimestampProto(time.Now()),
			Message:   "created",
		},
		DesiredState: api.TaskState_RUNNING,
		// TODO: Add Network attachment.
	}

	if err := ra.store.Update(func(tx store.Tx) error {
		return store.CreateTask(tx, t)
	}); err != nil {
		return nil, err
	}

	return &api.AttachNetworkResponse{AttachmentId: t.Id}, nil
}

// DetachNetwork allows the node to request the release of
// the resources associated to the network attachment.
// - Returns `InvalidArgument` if attachment ID is not provided.
// - Returns `NotFound` if the attachment is not found.
// - Returns an error if the deletion fails.
func (ra *ResourceAllocator) DetachNetwork(ctx context.Context, request *api.DetachNetworkRequest) (*api.DetachNetworkResponse, error) {
	if request.AttachmentId == "" {
		return nil, status.Error(codes.InvalidArgument, errInvalidArgument.Error())
	}

	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return nil, err
	}

	if err := ra.store.Update(func(tx store.Tx) error {
		t := store.GetTask(tx, request.AttachmentId)
		if t == nil {
			return status.Errorf(codes.NotFound, "attachment %s not found", request.AttachmentId)
		}
		if t.NodeId != nodeInfo.NodeID {
			return status.Errorf(codes.PermissionDenied, "attachment %s doesn't belong to this node", request.AttachmentId)
		}

		return store.DeleteTask(tx, request.AttachmentId)
	}); err != nil {
		return nil, err
	}

	return &api.DetachNetworkResponse{}, nil
}
