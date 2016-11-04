package agent

import (
	"github.com/docker/swarmkit/api"
	"golang.org/x/net/context"
)

type resourceAllocator struct {
	agent *Agent
}

// ResourceAllocator is an interface to allocate resource such as
// network attachments from a worker node.
type ResourceAllocator interface {
	// AttachNetwork creates a network attachment in the manager
	// given a target network and a unique ID representing the
	// connecting entity and optionally a list of ipv4/ipv6
	// addresses to be assigned to the attachment. AttachNetwork
	// returns a unique ID for the attachment if successful or an
	// error in case of failure.
	AttachNetwork(ctx context.Context, id, target string, addresses []string) (string, error)

	// DetachNetworks deletes a network attachment for the passed
	// attachment ID. The attachment ID is obtained from a
	// previous AttachNetwork call.
	DetachNetwork(ctx context.Context, aID string) error
}

// AttachNetwork creates a network attachment.
func (r *resourceAllocator) AttachNetwork(ctx context.Context, id, target string, addresses []string) (string, error) {
	var taskID string
	if err := r.agent.withSession(ctx, func(session *session) error {
		client := api.NewResourceAllocatorClient(session.conn)
		r, err := client.AttachNetwork(ctx, &api.AttachNetworkRequest{
			Config: &api.NetworkAttachmentConfig{
				Target:    target,
				Addresses: addresses,
			},
			ContainerID: id,
		})
		if err != nil {
			return err
		}
		taskID = r.AttachmentID
		return nil
	}); err != nil {
		return "", err
	}

	return taskID, nil
}

// DetachNetwork deletes a network attachment.
func (r *resourceAllocator) DetachNetwork(ctx context.Context, aID string) error {
	return r.agent.withSession(ctx, func(session *session) error {
		client := api.NewResourceAllocatorClient(session.conn)
		_, err := client.DetachNetwork(ctx, &api.DetachNetworkRequest{
			AttachmentID: aID,
		})

		return err
	})
}

// ResourceAllocator provides an interface to access resource
// allocation methods such as AttachNetwork and DetachNetwork.
func (a *Agent) ResourceAllocator() ResourceAllocator {
	return &resourceAllocator{agent: a}
}
