package resourceapi

import (
	"errors"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var (
	// ErrInvalidArgument returned when the request contains invalid arguments.
	ErrInvalidArgument = errors.New("invalid argument")
	// ProbeAttachment is a label to denote a probe network attachment
	ProbeAttachment = "ProbeAttachment"
)

// ResourceAllocator handles resource allocation of cluster entities.
type ResourceAllocator struct {
	store *store.MemoryStore
}

// New returns an instance of the allocator
func New(store *store.MemoryStore) *ResourceAllocator {
	return &ResourceAllocator{store: store}
}

// CreateNetworkAttachment allows the node to request the resources
// allocation needed for a network attachment on the specific node.
// - Returns `InvalidArgument` if the Spec is malformed.
// - Returns `NotFound` if the Network is not found.
// - Returns an error if the creation fails.
func (ra *ResourceAllocator) CreateNetworkAttachment(ctx context.Context, request *api.CreateNetworkAttachmentRequest) (*api.CreateNetworkAttachmentResponse, error) {
	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return nil, err
	}

	nodeID := nodeInfo.NodeID
	if nodeID != request.Config.NodeID {
		return nil, grpc.Errorf(codes.InvalidArgument, "network attachments can only be allocated on the same swarm node")
	}

	att := &api.NetworkAttachment{
		ID:     identity.NewID(),
		NodeID: request.Config.NodeID,
		Config: *request.Config,
	}

	ra.store.View(func(tx store.ReadTx) {
		att.Network = store.GetNetwork(tx, att.Config.Target)
		if att.Network == nil {
			if networks, err := store.FindNetworks(tx, store.ByName(att.Config.Target)); err == nil && len(networks) > 0 {
				att.Network = networks[0]
			}
		}
	})
	if att.Network == nil || !att.Network.Spec.Legacy {
		return nil, grpc.Errorf(codes.NotFound, "network  %s not found", att.Config.Target)
	}

	for _, addr := range att.Config.Addresses {
		att.Addresses = append(att.Addresses, addr)
	}

	if err := ra.store.Update(func(tx store.Tx) error {
		return store.CreateAttachment(tx, att)
	}); err != nil {
		return nil, err
	}

	log.G(ctx).Debugf("Executor Attachment %s created and saved to store", att.ID)

	return &api.CreateNetworkAttachmentResponse{ID: att.ID}, nil
}

// RemoveNetworkAttachment allows the node to request the release of
// the resources associated to the network attachment.
// - Returns `InvalidArgument` if attachment ID is not provided.
// - Returns `NotFound` if the attachment is not found.
// - Returns an error if the deletion fails.
func (ra *ResourceAllocator) RemoveNetworkAttachment(ctx context.Context, request *api.RemoveNetworkAttachmentRequest) (*api.RemoveNetworkAttachmentResponse, error) {
	if request.ID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, ErrInvalidArgument.Error())
	}

	if err := ra.store.Update(func(tx store.Tx) error {
		return store.DeleteAttachment(tx, request.ID)
	}); err != nil {
		if err == store.ErrNotExist {
			return nil, grpc.Errorf(codes.NotFound, "attachment %s not found", request.ID)
		}
		return nil, err
	}

	log.G(ctx).Debugf("Executor Attachment %s removed from store", request.ID)

	return &api.RemoveNetworkAttachmentResponse{}, nil
}
