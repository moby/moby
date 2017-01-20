package allocator

import (
	"fmt"
	"time"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/allocator/networkallocator"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

const (
	// Network allocator Voter ID for task allocation vote.
	networkVoter = "network"

	ingressNetworkName = "ingress"
	ingressSubnet      = "10.255.0.0/16"

	allocatedStatusMessage = "pending task scheduling"
)

var errNoChanges = errors.New("task unchanged")

func newIngressNetwork() *api.Network {
	return &api.Network{
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: ingressNetworkName,
				Labels: map[string]string{
					"com.docker.swarm.internal": "true",
				},
			},
			DriverConfig: &api.Driver{},
			IPAM: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet: ingressSubnet,
					},
				},
			},
		},
	}
}

// Network context information which is used throughout the network allocation code.
type networkContext struct {
	ingressNetwork *api.Network
	// Instance of the low-level network allocator which performs
	// the actual network allocation.
	nwkAllocator *networkallocator.NetworkAllocator

	// A table of unallocated tasks which will be revisited if any thing
	// changes in system state that might help task allocation.
	unallocatedTasks map[string]*api.Task

	// A table of unallocated services which will be revisited if
	// any thing changes in system state that might help service
	// allocation.
	unallocatedServices map[string]*api.Service

	// A table of unallocated networks which will be revisited if
	// any thing changes in system state that might help network
	// allocation.
	unallocatedNetworks map[string]*api.Network
}

func (a *Allocator) doNetworkInit(ctx context.Context) (err error) {
	na, err := networkallocator.New(a.pluginGetter)
	if err != nil {
		return err
	}

	nc := &networkContext{
		nwkAllocator:        na,
		unallocatedTasks:    make(map[string]*api.Task),
		unallocatedServices: make(map[string]*api.Service),
		unallocatedNetworks: make(map[string]*api.Network),
		ingressNetwork:      newIngressNetwork(),
	}
	a.netCtx = nc
	defer func() {
		// Clear a.netCtx if initialization was unsuccessful.
		if err != nil {
			a.netCtx = nil
		}
	}()

	// Check if we have the ingress network. If not found create
	// it before reading all network objects for allocation.
	var networks []*api.Network
	a.store.View(func(tx store.ReadTx) {
		networks, err = store.FindNetworks(tx, store.ByName(ingressNetworkName))
		if len(networks) > 0 {
			nc.ingressNetwork = networks[0]
		}
	})
	if err != nil {
		return errors.Wrap(err, "failed to find ingress network during init")
	}

	// If ingress network is not found, create one right away
	// using the predefined template.
	if len(networks) == 0 {
		if err := a.store.Update(func(tx store.Tx) error {
			nc.ingressNetwork.ID = identity.NewID()
			if err := store.CreateNetwork(tx, nc.ingressNetwork); err != nil {
				return err
			}

			return nil
		}); err != nil {
			return errors.Wrap(err, "failed to create ingress network")
		}

		a.store.View(func(tx store.ReadTx) {
			networks, err = store.FindNetworks(tx, store.ByName(ingressNetworkName))
			if len(networks) > 0 {
				nc.ingressNetwork = networks[0]
			}
		})
		if err != nil {
			return errors.Wrap(err, "failed to find ingress network after creating it")
		}

	}

	// Try to complete ingress network allocation before anything else so
	// that the we can get the preferred subnet for ingress
	// network.
	if !na.IsAllocated(nc.ingressNetwork) {
		if err := a.allocateNetwork(ctx, nc.ingressNetwork); err != nil {
			log.G(ctx).WithError(err).Error("failed allocating ingress network during init")
		} else if _, err := a.store.Batch(func(batch *store.Batch) error {
			if err := a.commitAllocatedNetwork(ctx, batch, nc.ingressNetwork); err != nil {
				log.G(ctx).WithError(err).Error("failed committing allocation of ingress network during init")
			}
			return nil
		}); err != nil {
			log.G(ctx).WithError(err).Error("failed committing allocation of ingress network during init")
		}
	}

	// Allocate networks in the store so far before we started
	// watching.
	a.store.View(func(tx store.ReadTx) {
		networks, err = store.FindNetworks(tx, store.All)
	})
	if err != nil {
		return errors.Wrap(err, "error listing all networks in store while trying to allocate during init")
	}

	var allocatedNetworks []*api.Network
	for _, n := range networks {
		if na.IsAllocated(n) {
			continue
		}

		if err := a.allocateNetwork(ctx, n); err != nil {
			log.G(ctx).WithError(err).Errorf("failed allocating network %s during init", n.ID)
			continue
		}
		allocatedNetworks = append(allocatedNetworks, n)
	}

	if _, err := a.store.Batch(func(batch *store.Batch) error {
		for _, n := range allocatedNetworks {
			if err := a.commitAllocatedNetwork(ctx, batch, n); err != nil {
				log.G(ctx).WithError(err).Errorf("failed committing allocation of network %s during init", n.ID)
			}
		}
		return nil
	}); err != nil {
		log.G(ctx).WithError(err).Error("failed committing allocation of networks during init")
	}

	// Allocate nodes in the store so far before we process watched events.
	var nodes []*api.Node
	a.store.View(func(tx store.ReadTx) {
		nodes, err = store.FindNodes(tx, store.All)
	})
	if err != nil {
		return errors.Wrap(err, "error listing all nodes in store while trying to allocate during init")
	}

	var allocatedNodes []*api.Node
	for _, node := range nodes {
		if na.IsNodeAllocated(node) {
			continue
		}

		if node.Attachment == nil {
			node.Attachment = &api.NetworkAttachment{}
		}

		node.Attachment.Network = nc.ingressNetwork.Copy()
		if err := a.allocateNode(ctx, node); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed to allocate network resources for node %s during init", node.ID)
			continue
		}

		allocatedNodes = append(allocatedNodes, node)
	}

	if _, err := a.store.Batch(func(batch *store.Batch) error {
		for _, node := range allocatedNodes {
			if err := a.commitAllocatedNode(ctx, batch, node); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to commit allocation of network resources for node %s during init", node.ID)
			}
		}
		return nil
	}); err != nil {
		log.G(ctx).WithError(err).Error("Failed to commit allocation of network resources for nodes during init")
	}

	// Allocate services in the store so far before we process watched events.
	var services []*api.Service
	a.store.View(func(tx store.ReadTx) {
		services, err = store.FindServices(tx, store.All)
	})
	if err != nil {
		return errors.Wrap(err, "error listing all services in store while trying to allocate during init")
	}

	var allocatedServices []*api.Service
	for _, s := range services {
		if nc.nwkAllocator.IsServiceAllocated(s) {
			continue
		}

		if err := a.allocateService(ctx, s); err != nil {
			log.G(ctx).WithError(err).Errorf("failed allocating service %s during init", s.ID)
			continue
		}
		allocatedServices = append(allocatedServices, s)
	}

	if _, err := a.store.Batch(func(batch *store.Batch) error {
		for _, s := range allocatedServices {
			if err := a.commitAllocatedService(ctx, batch, s); err != nil {
				log.G(ctx).WithError(err).Errorf("failed committing allocation of service %s during init", s.ID)
			}
		}
		return nil
	}); err != nil {
		log.G(ctx).WithError(err).Error("failed committing allocation of services during init")
	}

	// Allocate tasks in the store so far before we started watching.
	var (
		tasks          []*api.Task
		allocatedTasks []*api.Task
	)
	a.store.View(func(tx store.ReadTx) {
		tasks, err = store.FindTasks(tx, store.All)
	})
	if err != nil {
		return errors.Wrap(err, "error listing all tasks in store while trying to allocate during init")
	}

	for _, t := range tasks {
		if taskDead(t) {
			continue
		}

		var s *api.Service
		if t.ServiceID != "" {
			a.store.View(func(tx store.ReadTx) {
				s = store.GetService(tx, t.ServiceID)
			})
		}

		// Populate network attachments in the task
		// based on service spec.
		a.taskCreateNetworkAttachments(t, s)

		if taskReadyForNetworkVote(t, s, nc) {
			if t.Status.State >= api.TaskStatePending {
				continue
			}

			if a.taskAllocateVote(networkVoter, t.ID) {
				// If the task is not attached to any network, network
				// allocators job is done. Immediately cast a vote so
				// that the task can be moved to ALLOCATED state as
				// soon as possible.
				allocatedTasks = append(allocatedTasks, t)
			}
			continue
		}

		err := a.allocateTask(ctx, t)
		if err == nil {
			allocatedTasks = append(allocatedTasks, t)
		} else if err != errNoChanges {
			log.G(ctx).WithError(err).Errorf("failed allocating task %s during init", t.ID)
			nc.unallocatedTasks[t.ID] = t
		}
	}

	if _, err := a.store.Batch(func(batch *store.Batch) error {
		for _, t := range allocatedTasks {
			if err := a.commitAllocatedTask(ctx, batch, t); err != nil {
				log.G(ctx).WithError(err).Errorf("failed committing allocation of task %s during init", t.ID)
			}
		}

		return nil
	}); err != nil {
		log.G(ctx).WithError(err).Error("failed committing allocation of tasks during init")
	}

	return nil
}

func (a *Allocator) doNetworkAlloc(ctx context.Context, ev events.Event) {
	nc := a.netCtx

	switch v := ev.(type) {
	case state.EventCreateNetwork:
		n := v.Network.Copy()
		if nc.nwkAllocator.IsAllocated(n) {
			break
		}

		if err := a.allocateNetwork(ctx, n); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed allocation for network %s", n.ID)
			break
		}

		if _, err := a.store.Batch(func(batch *store.Batch) error {
			return a.commitAllocatedNetwork(ctx, batch, n)
		}); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed to commit allocation for network %s", n.ID)
		}
	case state.EventDeleteNetwork:
		n := v.Network.Copy()

		// The assumption here is that all dependent objects
		// have been cleaned up when we are here so the only
		// thing that needs to happen is free the network
		// resources.
		if err := nc.nwkAllocator.Deallocate(n); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed during network free for network %s", n.ID)
		}
	case state.EventCreateService:
		s := v.Service.Copy()

		if nc.nwkAllocator.IsServiceAllocated(s) {
			break
		}

		if err := a.allocateService(ctx, s); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed allocation for service %s", s.ID)
			break
		}

		if _, err := a.store.Batch(func(batch *store.Batch) error {
			return a.commitAllocatedService(ctx, batch, s)
		}); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed to commit allocation for service %s", s.ID)
		}
	case state.EventUpdateService:
		s := v.Service.Copy()

		if nc.nwkAllocator.IsServiceAllocated(s) {
			break
		}

		if err := a.allocateService(ctx, s); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed allocation during update of service %s", s.ID)
			break
		}

		if _, err := a.store.Batch(func(batch *store.Batch) error {
			return a.commitAllocatedService(ctx, batch, s)
		}); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed to commit allocation during update for service %s", s.ID)
		}
	case state.EventDeleteService:
		s := v.Service.Copy()

		if err := nc.nwkAllocator.ServiceDeallocate(s); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed deallocation during delete of service %s", s.ID)
		}

		// Remove it from unallocatedServices just in case
		// it's still there.
		delete(nc.unallocatedServices, s.ID)
	case state.EventCreateNode, state.EventUpdateNode, state.EventDeleteNode:
		a.doNodeAlloc(ctx, ev)
	case state.EventCreateTask, state.EventUpdateTask, state.EventDeleteTask:
		a.doTaskAlloc(ctx, ev)
	case state.EventCommit:
		a.procUnallocatedNetworks(ctx)
		a.procUnallocatedServices(ctx)
		a.procUnallocatedTasksNetwork(ctx)
		return
	}
}

func (a *Allocator) doNodeAlloc(ctx context.Context, ev events.Event) {
	var (
		isDelete bool
		node     *api.Node
	)

	switch v := ev.(type) {
	case state.EventCreateNode:
		node = v.Node.Copy()
	case state.EventUpdateNode:
		node = v.Node.Copy()
	case state.EventDeleteNode:
		isDelete = true
		node = v.Node.Copy()
	}

	nc := a.netCtx

	if isDelete {
		if nc.nwkAllocator.IsNodeAllocated(node) {
			if err := nc.nwkAllocator.DeallocateNode(node); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed freeing network resources for node %s", node.ID)
			}
		}
		return
	}

	if !nc.nwkAllocator.IsNodeAllocated(node) {
		if node.Attachment == nil {
			node.Attachment = &api.NetworkAttachment{}
		}

		node.Attachment.Network = nc.ingressNetwork.Copy()
		if err := a.allocateNode(ctx, node); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed to allocate network resources for node %s", node.ID)
			return
		}

		if _, err := a.store.Batch(func(batch *store.Batch) error {
			return a.commitAllocatedNode(ctx, batch, node)
		}); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed to commit allocation of network resources for node %s", node.ID)
		}
	}
}

// taskRunning checks whether a task is either actively running, or in the
// process of starting up.
func taskRunning(t *api.Task) bool {
	return t.DesiredState <= api.TaskStateRunning && t.Status.State <= api.TaskStateRunning
}

// taskDead checks whether a task is not actively running as far as allocator purposes are concerned.
func taskDead(t *api.Task) bool {
	return t.DesiredState > api.TaskStateRunning && t.Status.State > api.TaskStateRunning
}

// taskReadyForNetworkVote checks if the task is ready for a network
// vote to move it to ALLOCATED state.
func taskReadyForNetworkVote(t *api.Task, s *api.Service, nc *networkContext) bool {
	// Task is ready for vote if the following is true:
	//
	// Task has no network attached or networks attached but all
	// of them allocated AND Task's service has no endpoint or
	// network configured or service endpoints have been
	// allocated.
	return (len(t.Networks) == 0 || nc.nwkAllocator.IsTaskAllocated(t)) &&
		(s == nil || nc.nwkAllocator.IsServiceAllocated(s))
}

func taskUpdateNetworks(t *api.Task, networks []*api.NetworkAttachment) {
	networksCopy := make([]*api.NetworkAttachment, 0, len(networks))
	for _, n := range networks {
		networksCopy = append(networksCopy, n.Copy())
	}

	t.Networks = networksCopy
}

func taskUpdateEndpoint(t *api.Task, endpoint *api.Endpoint) {
	t.Endpoint = endpoint.Copy()
}

func isIngressNetworkNeeded(s *api.Service) bool {
	if s == nil {
		return false
	}

	if s.Spec.Endpoint == nil {
		return false
	}

	for _, p := range s.Spec.Endpoint.Ports {
		// The service to which this task belongs is trying to
		// expose ports with PublishMode as Ingress to the
		// external world. Automatically attach the task to
		// the ingress network.
		if p.PublishMode == api.PublishModeIngress {
			return true
		}
	}

	return false
}

func (a *Allocator) taskCreateNetworkAttachments(t *api.Task, s *api.Service) {
	// If task network attachments have already been filled in no
	// need to do anything else.
	if len(t.Networks) != 0 {
		return
	}

	var networks []*api.NetworkAttachment
	if isIngressNetworkNeeded(s) {
		networks = append(networks, &api.NetworkAttachment{Network: a.netCtx.ingressNetwork})
	}

	a.store.View(func(tx store.ReadTx) {
		// Always prefer NetworkAttachmentConfig in the TaskSpec
		specNetworks := t.Spec.Networks
		if len(specNetworks) == 0 && s != nil && len(s.Spec.Networks) != 0 {
			specNetworks = s.Spec.Networks
		}

		for _, na := range specNetworks {
			n := store.GetNetwork(tx, na.Target)
			if n == nil {
				continue
			}

			attachment := api.NetworkAttachment{Network: n}
			attachment.Aliases = append(attachment.Aliases, na.Aliases...)
			attachment.Addresses = append(attachment.Addresses, na.Addresses...)

			networks = append(networks, &attachment)
		}
	})

	taskUpdateNetworks(t, networks)
}

func (a *Allocator) doTaskAlloc(ctx context.Context, ev events.Event) {
	var (
		isDelete bool
		t        *api.Task
	)

	switch v := ev.(type) {
	case state.EventCreateTask:
		t = v.Task.Copy()
	case state.EventUpdateTask:
		t = v.Task.Copy()
	case state.EventDeleteTask:
		isDelete = true
		t = v.Task.Copy()
	}

	nc := a.netCtx

	// If the task has stopped running or it's being deleted then
	// we should free the network resources associated with the
	// task right away.
	if taskDead(t) || isDelete {
		if nc.nwkAllocator.IsTaskAllocated(t) {
			if err := nc.nwkAllocator.DeallocateTask(t); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed freeing network resources for task %s", t.ID)
			}
		}

		// Cleanup any task references that might exist in unallocatedTasks
		delete(nc.unallocatedTasks, t.ID)
		return
	}

	// If we are already in allocated state, there is
	// absolutely nothing else to do.
	if t.Status.State >= api.TaskStatePending {
		delete(nc.unallocatedTasks, t.ID)
		return
	}

	var s *api.Service
	if t.ServiceID != "" {
		a.store.View(func(tx store.ReadTx) {
			s = store.GetService(tx, t.ServiceID)
		})
		if s == nil {
			// If the task is running it is not normal to
			// not be able to find the associated
			// service. If the task is not running (task
			// is either dead or the desired state is set
			// to dead) then the service may not be
			// available in store. But we still need to
			// cleanup network resources associated with
			// the task.
			if taskRunning(t) && !isDelete {
				log.G(ctx).Errorf("Event %T: Failed to get service %s for task %s state %s: could not find service %s", ev, t.ServiceID, t.ID, t.Status.State, t.ServiceID)
				return
			}
		}
	}

	// Populate network attachments in the task
	// based on service spec.
	a.taskCreateNetworkAttachments(t, s)

	nc.unallocatedTasks[t.ID] = t
}

func (a *Allocator) allocateNode(ctx context.Context, node *api.Node) error {
	return a.netCtx.nwkAllocator.AllocateNode(node)
}

func (a *Allocator) commitAllocatedNode(ctx context.Context, batch *store.Batch, node *api.Node) error {
	if err := batch.Update(func(tx store.Tx) error {
		err := store.UpdateNode(tx, node)

		if err == store.ErrSequenceConflict {
			storeNode := store.GetNode(tx, node.ID)
			storeNode.Attachment = node.Attachment.Copy()
			err = store.UpdateNode(tx, storeNode)
		}

		return errors.Wrapf(err, "failed updating state in store transaction for node %s", node.ID)
	}); err != nil {
		if err := a.netCtx.nwkAllocator.DeallocateNode(node); err != nil {
			log.G(ctx).WithError(err).Errorf("failed rolling back allocation of node %s", node.ID)
		}

		return err
	}

	return nil
}

func (a *Allocator) allocateService(ctx context.Context, s *api.Service) error {
	nc := a.netCtx

	if s.Spec.Endpoint != nil {
		// service has user-defined endpoint
		if s.Endpoint == nil {
			// service currently has no allocated endpoint, need allocated.
			s.Endpoint = &api.Endpoint{
				Spec: s.Spec.Endpoint.Copy(),
			}
		}

		// The service is trying to expose ports to the external
		// world. Automatically attach the service to the ingress
		// network only if it is not already done.
		if isIngressNetworkNeeded(s) {
			var found bool
			for _, vip := range s.Endpoint.VirtualIPs {
				if vip.NetworkID == nc.ingressNetwork.ID {
					found = true
					break
				}
			}

			if !found {
				s.Endpoint.VirtualIPs = append(s.Endpoint.VirtualIPs,
					&api.Endpoint_VirtualIP{NetworkID: nc.ingressNetwork.ID})
			}
		}
	} else if s.Endpoint != nil {
		// service has no user-defined endpoints while has already allocated network resources,
		// need deallocated.
		if err := nc.nwkAllocator.ServiceDeallocate(s); err != nil {
			return err
		}
	}

	if err := nc.nwkAllocator.ServiceAllocate(s); err != nil {
		nc.unallocatedServices[s.ID] = s
		return err
	}

	// If the service doesn't expose ports any more and if we have
	// any lingering virtual IP references for ingress network
	// clean them up here.
	if !isIngressNetworkNeeded(s) {
		if s.Endpoint != nil {
			for i, vip := range s.Endpoint.VirtualIPs {
				if vip.NetworkID == nc.ingressNetwork.ID {
					n := len(s.Endpoint.VirtualIPs)
					s.Endpoint.VirtualIPs[i], s.Endpoint.VirtualIPs[n-1] = s.Endpoint.VirtualIPs[n-1], nil
					s.Endpoint.VirtualIPs = s.Endpoint.VirtualIPs[:n-1]
					break
				}
			}
		}
	}
	return nil
}

func (a *Allocator) commitAllocatedService(ctx context.Context, batch *store.Batch, s *api.Service) error {
	if err := batch.Update(func(tx store.Tx) error {
		err := store.UpdateService(tx, s)

		if err == store.ErrSequenceConflict {
			storeService := store.GetService(tx, s.ID)
			storeService.Endpoint = s.Endpoint
			err = store.UpdateService(tx, storeService)
		}

		return errors.Wrapf(err, "failed updating state in store transaction for service %s", s.ID)
	}); err != nil {
		if err := a.netCtx.nwkAllocator.ServiceDeallocate(s); err != nil {
			log.G(ctx).WithError(err).Errorf("failed rolling back allocation of service %s", s.ID)
		}

		return err
	}

	return nil
}

func (a *Allocator) allocateNetwork(ctx context.Context, n *api.Network) error {
	nc := a.netCtx

	if err := nc.nwkAllocator.Allocate(n); err != nil {
		nc.unallocatedNetworks[n.ID] = n
		return errors.Wrapf(err, "failed during network allocation for network %s", n.ID)
	}

	return nil
}

func (a *Allocator) commitAllocatedNetwork(ctx context.Context, batch *store.Batch, n *api.Network) error {
	if err := batch.Update(func(tx store.Tx) error {
		if err := store.UpdateNetwork(tx, n); err != nil {
			return errors.Wrapf(err, "failed updating state in store transaction for network %s", n.ID)
		}
		return nil
	}); err != nil {
		if err := a.netCtx.nwkAllocator.Deallocate(n); err != nil {
			log.G(ctx).WithError(err).Errorf("failed rolling back allocation of network %s", n.ID)
		}

		return err
	}

	return nil
}

func (a *Allocator) allocateTask(ctx context.Context, t *api.Task) (err error) {
	taskUpdated := false
	nc := a.netCtx

	// We might be here even if a task allocation has already
	// happened but wasn't successfully committed to store. In such
	// cases skip allocation and go straight ahead to updating the
	// store.
	if !nc.nwkAllocator.IsTaskAllocated(t) {
		a.store.View(func(tx store.ReadTx) {
			if t.ServiceID != "" {
				s := store.GetService(tx, t.ServiceID)
				if s == nil {
					err = fmt.Errorf("could not find service %s", t.ServiceID)
					return
				}

				if !nc.nwkAllocator.IsServiceAllocated(s) {
					err = fmt.Errorf("service %s to which this task %s belongs has pending allocations", s.ID, t.ID)
					return
				}

				if s.Endpoint != nil {
					taskUpdateEndpoint(t, s.Endpoint)
					taskUpdated = true
				}
			}

			for _, na := range t.Networks {
				n := store.GetNetwork(tx, na.Network.ID)
				if n == nil {
					err = fmt.Errorf("failed to retrieve network %s while allocating task %s", na.Network.ID, t.ID)
					return
				}

				if !nc.nwkAllocator.IsAllocated(n) {
					err = fmt.Errorf("network %s attached to task %s not allocated yet", n.ID, t.ID)
					return
				}

				na.Network = n
			}

			if err = nc.nwkAllocator.AllocateTask(t); err != nil {
				err = errors.Wrapf(err, "failed during networktask allocation for task %s", t.ID)
				return
			}
			if nc.nwkAllocator.IsTaskAllocated(t) {
				taskUpdated = true
			}
		})

		if err != nil {
			return err
		}
	}

	// Update the network allocations and moving to
	// PENDING state on top of the latest store state.
	if a.taskAllocateVote(networkVoter, t.ID) {
		if t.Status.State < api.TaskStatePending {
			updateTaskStatus(t, api.TaskStatePending, allocatedStatusMessage)
			taskUpdated = true
		}
	}

	if !taskUpdated {
		return errNoChanges
	}

	return nil
}

func (a *Allocator) commitAllocatedTask(ctx context.Context, batch *store.Batch, t *api.Task) error {
	return batch.Update(func(tx store.Tx) error {
		err := store.UpdateTask(tx, t)

		if err == store.ErrSequenceConflict {
			storeTask := store.GetTask(tx, t.ID)
			taskUpdateNetworks(storeTask, t.Networks)
			taskUpdateEndpoint(storeTask, t.Endpoint)
			if storeTask.Status.State < api.TaskStatePending {
				storeTask.Status = t.Status
			}
			err = store.UpdateTask(tx, storeTask)
		}

		return errors.Wrapf(err, "failed updating state in store transaction for task %s", t.ID)
	})
}

func (a *Allocator) procUnallocatedNetworks(ctx context.Context) {
	nc := a.netCtx
	var allocatedNetworks []*api.Network
	for _, n := range nc.unallocatedNetworks {
		if !nc.nwkAllocator.IsAllocated(n) {
			if err := a.allocateNetwork(ctx, n); err != nil {
				log.G(ctx).WithError(err).Debugf("Failed allocation of unallocated network %s", n.ID)
				continue
			}
			allocatedNetworks = append(allocatedNetworks, n)
		}
	}

	if len(allocatedNetworks) == 0 {
		return
	}

	committed, err := a.store.Batch(func(batch *store.Batch) error {
		for _, n := range allocatedNetworks {
			if err := a.commitAllocatedNetwork(ctx, batch, n); err != nil {
				log.G(ctx).WithError(err).Debugf("Failed to commit allocation of unallocated network %s", n.ID)
				continue
			}
		}
		return nil
	})

	if err != nil {
		log.G(ctx).WithError(err).Error("Failed to commit allocation of unallocated networks")
	}

	for _, n := range allocatedNetworks[:committed] {
		delete(nc.unallocatedNetworks, n.ID)
	}
}

func (a *Allocator) procUnallocatedServices(ctx context.Context) {
	nc := a.netCtx
	var allocatedServices []*api.Service
	for _, s := range nc.unallocatedServices {
		if !nc.nwkAllocator.IsServiceAllocated(s) {
			if err := a.allocateService(ctx, s); err != nil {
				log.G(ctx).WithError(err).Debugf("Failed allocation of unallocated service %s", s.ID)
				continue
			}
			allocatedServices = append(allocatedServices, s)
		}
	}

	if len(allocatedServices) == 0 {
		return
	}

	committed, err := a.store.Batch(func(batch *store.Batch) error {
		for _, s := range allocatedServices {
			if err := a.commitAllocatedService(ctx, batch, s); err != nil {
				log.G(ctx).WithError(err).Debugf("Failed to commit allocation of unallocated service %s", s.ID)
				continue
			}
		}
		return nil
	})

	if err != nil {
		log.G(ctx).WithError(err).Error("Failed to commit allocation of unallocated services")
	}

	for _, s := range allocatedServices[:committed] {
		delete(nc.unallocatedServices, s.ID)
	}
}

func (a *Allocator) procUnallocatedTasksNetwork(ctx context.Context) {
	nc := a.netCtx
	allocatedTasks := make([]*api.Task, 0, len(nc.unallocatedTasks))

	for _, t := range nc.unallocatedTasks {
		if err := a.allocateTask(ctx, t); err == nil {
			allocatedTasks = append(allocatedTasks, t)
		} else if err != errNoChanges {
			log.G(ctx).WithError(err).Error("task allocation failure")
		}
	}

	if len(allocatedTasks) == 0 {
		return
	}

	committed, err := a.store.Batch(func(batch *store.Batch) error {
		for _, t := range allocatedTasks {
			err := a.commitAllocatedTask(ctx, batch, t)

			if err != nil {
				log.G(ctx).WithError(err).Error("task allocation commit failure")
				continue
			}
		}

		return nil
	})

	if err != nil {
		log.G(ctx).WithError(err).Error("failed a store batch operation while processing unallocated tasks")
	}

	for _, t := range allocatedTasks[:committed] {
		delete(nc.unallocatedTasks, t.ID)
	}
}

// updateTaskStatus sets TaskStatus and updates timestamp.
func updateTaskStatus(t *api.Task, newStatus api.TaskState, message string) {
	t.Status.State = newStatus
	t.Status.Message = message
	t.Status.Timestamp = ptypes.MustTimestampProto(time.Now())
}
