package cluster

import (
	"context"
	"fmt"

	"github.com/containerd/log"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/moby/moby/api/types/network"
	types "github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/v2/daemon/cluster/convert"
	"github.com/moby/moby/v2/daemon/cluster/convert/netextra"
	networkSettings "github.com/moby/moby/v2/daemon/network"
	"github.com/moby/moby/v2/errdefs"
	swarmapi "github.com/moby/swarmkit/v2/api"
	"github.com/pkg/errors"
)

// GetNetworks returns all current cluster managed networks.
func (c *Cluster) GetNetworks(filter networkSettings.Filter, withStatus bool) ([]network.Inspect, error) {
	// Swarmkit API's filters are too limited to express the Moby filter
	// semantics with much fidelity. It only supports filtering on one of:
	//  - Names (exact match)
	//  - NamePrefixes (prefix match)
	//  - IDPrefixes (prefix match)
	// The first of the list that is set is used as the filter predicate.
	// The other fields are ignored. However, the Engine API filter
	// semantics are to match on any substring of the network name or ID. We
	// therefore need to request all networks from Swarmkit and filter them
	// ourselves.
	list, err := c.listNetworks(context.TODO(), nil, withStatus)
	if err != nil {
		return nil, err
	}
	var filtered []network.Inspect
	for _, n := range list {
		if n.Spec.Annotations.Labels["com.docker.swarm.predefined"] == "true" {
			continue
		}
		if filter.Matches(convert.FilterNetwork{N: n}) {
			nn, err := convert.NetworkInspectFromGRPC(*n)
			if err != nil {
				return nil, fmt.Errorf("%s: failed to convert swarmapi.Network to network.Inspect: %w", n.ID, err)
			}
			filtered = append(filtered, nn)
		}
	}

	return filtered, nil
}

func (c *Cluster) GetNetworkSummaries(filter networkSettings.Filter) ([]network.Summary, error) {
	list, err := c.listNetworks(context.TODO(), nil, false)
	if err != nil {
		return nil, err
	}
	var filtered []network.Summary
	for _, n := range list {
		if n.Spec.Annotations.Labels["com.docker.swarm.predefined"] == "true" {
			continue
		}
		if filter.Matches(convert.FilterNetwork{N: n}) {
			filtered = append(filtered, network.Summary{Network: convert.BasicNetworkFromGRPC(*n)})
		}
	}

	return filtered, nil
}

func (c *Cluster) listNetworks(ctx context.Context, filters *swarmapi.ListNetworksRequest_Filters, withStatus bool) ([]*swarmapi.Network, error) {
	var appdata *gogotypes.Any
	if withStatus {
		var err error
		appdata, err = gogotypes.MarshalAny(&netextra.GetNetworkExtraOptions{
			WithIPAMStatus: withStatus,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal GetNetworkExtraOptions: %w", err)
		}
	}

	var list []*swarmapi.Network
	err := c.lockedManagerAction(ctx, func(ctx context.Context, state nodeState) error {
		l, err := state.controlClient.ListNetworks(ctx, &swarmapi.ListNetworksRequest{Filters: filters, Appdata: appdata})
		if err != nil {
			return err
		}
		list = l.Networks
		return nil
	})
	return list, err
}

// GetNetwork returns a cluster network by an ID.
func (c *Cluster) GetNetwork(input string, withStatus bool) (network.Inspect, error) {
	var appdata *gogotypes.Any
	if withStatus {
		var err error
		appdata, err = gogotypes.MarshalAny(&netextra.GetNetworkExtraOptions{
			WithIPAMStatus: withStatus,
		})
		if err != nil {
			return network.Inspect{}, fmt.Errorf("failed to marshal GetNetworkExtraOptions: %w", err)
		}
	}

	var nw *swarmapi.Network
	if err := c.lockedManagerAction(context.TODO(), func(ctx context.Context, state nodeState) error {
		n, err := getNetwork(ctx, state.controlClient, input, appdata)
		if err != nil {
			return err
		}
		nw = n
		return nil
	}); err != nil {
		return network.Inspect{}, err
	}
	return convert.NetworkInspectFromGRPC(*nw)
}

// GetNetworksByName returns cluster managed networks by name.
// It is ok to have multiple networks here. #18864
func (c *Cluster) GetNetworksByName(name string) ([]network.Network, error) {
	// Note that swarmapi.GetNetworkRequest.Name is not functional.
	// So we cannot just use that with c.GetNetwork.
	list, err := c.listNetworks(context.TODO(), &swarmapi.ListNetworksRequest_Filters{
		Names: []string{name},
	}, false)
	if err != nil {
		return nil, err
	}
	nr := make([]network.Network, len(list))
	for i, n := range list {
		nr[i] = convert.BasicNetworkFromGRPC(*n)
	}
	return nr, nil
}

func attacherKey(target, containerID string) string {
	return containerID + ":" + target
}

// UpdateAttachment signals the attachment config to the attachment
// waiter who is trying to start or attach the container to the
// network.
func (c *Cluster) UpdateAttachment(target, containerID string, config *network.NetworkingConfig) error {
	c.mu.Lock()
	attacher, ok := c.attachers[attacherKey(target, containerID)]
	if !ok || attacher == nil {
		c.mu.Unlock()
		return fmt.Errorf("could not find attacher for container %s to network %s", containerID, target)
	}
	if attacher.inProgress {
		log.G(context.TODO()).Debugf("Discarding redundant notice of resource allocation on network %s for task id %s", target, attacher.taskID)
		c.mu.Unlock()
		return nil
	}
	attacher.inProgress = true
	c.mu.Unlock()

	attacher.attachWaitCh <- config

	return nil
}

// WaitForDetachment waits for the container to stop or detach from
// the network.
func (c *Cluster) WaitForDetachment(ctx context.Context, networkName, networkID, taskID, containerID string) error {
	c.mu.RLock()
	attacher, ok := c.attachers[attacherKey(networkName, containerID)]
	if !ok {
		attacher, ok = c.attachers[attacherKey(networkID, containerID)]
	}
	state := c.currentNodeState()
	if state.swarmNode == nil || state.swarmNode.Agent() == nil {
		c.mu.RUnlock()
		return errors.New("invalid cluster node while waiting for detachment")
	}

	c.mu.RUnlock()
	agent := state.swarmNode.Agent()
	if ok && attacher != nil &&
		attacher.detachWaitCh != nil &&
		attacher.attachCompleteCh != nil {
		// Attachment may be in progress still so wait for
		// attachment to complete.
		select {
		case <-attacher.attachCompleteCh:
		case <-ctx.Done():
			return ctx.Err()
		}

		if attacher.taskID == taskID {
			select {
			case <-attacher.detachWaitCh:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return agent.ResourceAllocator().DetachNetwork(ctx, taskID)
}

// AttachNetwork generates an attachment request towards the manager.
func (c *Cluster) AttachNetwork(target string, containerID string, addresses []string) (*network.NetworkingConfig, error) {
	aKey := attacherKey(target, containerID)
	c.mu.Lock()
	state := c.currentNodeState()
	if state.swarmNode == nil || state.swarmNode.Agent() == nil {
		c.mu.Unlock()
		return nil, errors.New("invalid cluster node while attaching to network")
	}
	if attacher, ok := c.attachers[aKey]; ok {
		c.mu.Unlock()
		return attacher.config, nil
	}

	agent := state.swarmNode.Agent()
	attachWaitCh := make(chan *network.NetworkingConfig)
	detachWaitCh := make(chan struct{})
	attachCompleteCh := make(chan struct{})
	c.attachers[aKey] = &attacher{
		attachWaitCh:     attachWaitCh,
		attachCompleteCh: attachCompleteCh,
		detachWaitCh:     detachWaitCh,
	}
	c.mu.Unlock()

	ctx := context.TODO()
	ctx, cancel := context.WithTimeout(ctx, swarmRequestTimeout)
	defer cancel()

	taskID, err := agent.ResourceAllocator().AttachNetwork(ctx, containerID, target, addresses)
	if err != nil {
		c.mu.Lock()
		delete(c.attachers, aKey)
		c.mu.Unlock()
		return nil, fmt.Errorf("Could not attach to network %s: %v", target, err)
	}

	c.mu.Lock()
	c.attachers[aKey].taskID = taskID
	close(attachCompleteCh)
	c.mu.Unlock()

	log.G(ctx).Debugf("Successfully attached to network %s with task id %s", target, taskID)

	release := func() {
		ctx := context.WithoutCancel(ctx)
		ctx, cancel := context.WithTimeout(ctx, swarmRequestTimeout)
		defer cancel()
		if err := agent.ResourceAllocator().DetachNetwork(ctx, taskID); err != nil {
			log.G(ctx).Errorf("Failed remove network attachment %s to network %s on allocation failure: %v",
				taskID, target, err)
		}
	}

	var config *network.NetworkingConfig
	select {
	case config = <-attachWaitCh:
	case <-ctx.Done():
		release()
		return nil, fmt.Errorf("attaching to network failed, make sure your network options are correct and check manager logs: %v", ctx.Err())
	}

	c.mu.Lock()
	c.attachers[aKey].config = config
	c.mu.Unlock()

	log.G(ctx).Debugf("Successfully allocated resources on network %s for task id %s", target, taskID)

	return config, nil
}

// DetachNetwork unblocks the waiters waiting on WaitForDetachment so
// that a request to detach can be generated towards the manager.
func (c *Cluster) DetachNetwork(target string, containerID string) error {
	aKey := attacherKey(target, containerID)

	c.mu.Lock()
	attacher, ok := c.attachers[aKey]
	delete(c.attachers, aKey)
	c.mu.Unlock()

	if !ok {
		return fmt.Errorf("could not find network attachment for container %s to network %s", containerID, target)
	}

	close(attacher.detachWaitCh)
	return nil
}

// CreateNetwork creates a new cluster managed network.
func (c *Cluster) CreateNetwork(s network.CreateRequest) (string, error) {
	if networkSettings.IsPredefined(s.Name) {
		err := notAllowedError(fmt.Sprintf("%s is a pre-defined network and cannot be created", s.Name))
		return "", errors.WithStack(err)
	}

	var resp *swarmapi.CreateNetworkResponse
	if err := c.lockedManagerAction(context.TODO(), func(ctx context.Context, state nodeState) error {
		networkSpec := convert.BasicNetworkCreateToGRPC(s)
		r, err := state.controlClient.CreateNetwork(ctx, &swarmapi.CreateNetworkRequest{Spec: &networkSpec})
		if err != nil {
			return err
		}
		resp = r
		return nil
	}); err != nil {
		return "", err
	}

	return resp.Network.ID, nil
}

// RemoveNetwork removes a cluster network.
func (c *Cluster) RemoveNetwork(input string) error {
	return c.lockedManagerAction(context.TODO(), func(ctx context.Context, state nodeState) error {
		nw, err := getNetwork(ctx, state.controlClient, input, nil)
		if err != nil {
			return err
		}

		_, err = state.controlClient.RemoveNetwork(ctx, &swarmapi.RemoveNetworkRequest{NetworkID: nw.ID})
		return err
	})
}

func (c *Cluster) populateNetworkID(ctx context.Context, client swarmapi.ControlClient, s *types.ServiceSpec) error {
	networks := s.TaskTemplate.Networks
	for i, nw := range networks {
		apiNetwork, err := getNetwork(ctx, client, nw.Target, nil)
		if err != nil {
			ln, _ := c.config.Backend.FindNetwork(nw.Target)
			if ln != nil && networkSettings.IsPredefined(ln.Name()) {
				// Need to retrieve the corresponding predefined swarm network
				// and use its id for the request.
				apiNetwork, err = getNetwork(ctx, client, ln.Name(), nil)
				if err != nil {
					return errors.Wrap(errdefs.NotFound(err), "could not find the corresponding predefined swarm network")
				}
				goto setid
			}
			if ln != nil && !ln.Dynamic() {
				errMsg := fmt.Sprintf("The network %s cannot be used with services. Only networks scoped to the swarm can be used, such as those created with the overlay driver.", ln.Name())
				return errors.WithStack(notAllowedError(errMsg))
			}
			return err
		}
	setid:
		networks[i].Target = apiNetwork.ID
	}
	return nil
}
