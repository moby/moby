package allocator

import (
	"fmt"
	"time"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
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
	networkVoter           = "network"
	allocatedStatusMessage = "pending task scheduling"
)

var (
	// ErrNoIngress is returned when no ingress network is found in store
	ErrNoIngress = errors.New("no ingress network found")
	errNoChanges = errors.New("task unchanged")

	retryInterval = 5 * time.Minute
)

// Network context information which is used throughout the network allocation code.
type networkContext struct {
	ingressNetwork *api.Network
	// Instance of the low-level network allocator which performs
	// the actual network allocation.
	nwkAllocator *networkallocator.NetworkAllocator

	// A set of tasks which are ready to be allocated as a batch. This is
	// distinct from "unallocatedTasks" which are tasks that failed to
	// allocate on the first try, being held for a future retry.
	pendingTasks map[string]*api.Task

	// A set of unallocated tasks which will be revisited if any thing
	// changes in system state that might help task allocation.
	unallocatedTasks map[string]*api.Task

	// A set of unallocated services which will be revisited if
	// any thing changes in system state that might help service
	// allocation.
	unallocatedServices map[string]*api.Service

	// A set of unallocated networks which will be revisited if
	// any thing changes in system state that might help network
	// allocation.
	unallocatedNetworks map[string]*api.Network

	// lastRetry is the last timestamp when unallocated
	// tasks/services/networks were retried.
	lastRetry time.Time
}

func (a *Allocator) doNetworkInit(ctx context.Context) (err error) {
	na, err := networkallocator.New(a.pluginGetter)
	if err != nil {
		return err
	}

	nc := &networkContext{
		nwkAllocator:        na,
		pendingTasks:        make(map[string]*api.Task),
		unallocatedTasks:    make(map[string]*api.Task),
		unallocatedServices: make(map[string]*api.Service),
		unallocatedNetworks: make(map[string]*api.Network),
		lastRetry:           time.Now(),
	}
	a.netCtx = nc
	defer func() {
		// Clear a.netCtx if initialization was unsuccessful.
		if err != nil {
			a.netCtx = nil
		}
	}()

	// Ingress network is now created at cluster's first time creation.
	// Check if we have the ingress network. If found, make sure it is
	// allocated, before reading all network objects for allocation.
	// If not found, it means it was removed by user, nothing to do here.
	ingressNetwork, err := GetIngressNetwork(a.store)
	switch err {
	case nil:
		// Try to complete ingress network allocation before anything else so
		// that the we can get the preferred subnet for ingress network.
		nc.ingressNetwork = ingressNetwork
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
	case ErrNoIngress:
		// Ingress network is not present in store, It means user removed it
		// and did not create a new one.
	default:
		return errors.Wrap(err, "failure while looking for ingress network during init")
	}

	// Allocate networks in the store so far before we started
	// watching.
	var networks []*api.Network
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

	// Allocate nodes in the store so far before we process watched events,
	// if the ingress network is present.
	if nc.ingressNetwork != nil {
		if err := a.allocateNodes(ctx); err != nil {
			return err
		}
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
		if nc.nwkAllocator.IsServiceAllocated(s, networkallocator.OnInit) {
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
		if t.Status.State > api.TaskStateRunning {
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
				// that the task can be moved to the PENDING state as
				// soon as possible.
				updateTaskStatus(t, api.TaskStatePending, allocatedStatusMessage)
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
	case api.EventCreateNetwork:
		n := v.Network.Copy()
		if nc.nwkAllocator.IsAllocated(n) {
			break
		}

		if IsIngressNetwork(n) && nc.ingressNetwork != nil {
			log.G(ctx).Errorf("Cannot allocate ingress network %s (%s) because another ingress network is already present: %s (%s)",
				n.ID, n.Spec.Annotations.Name, nc.ingressNetwork.ID, nc.ingressNetwork.Spec.Annotations)
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

		if IsIngressNetwork(n) {
			nc.ingressNetwork = n
			err := a.allocateNodes(ctx)
			if err != nil {
				log.G(ctx).WithError(err).Error(err)
			}
		}
	case api.EventDeleteNetwork:
		n := v.Network.Copy()

		if IsIngressNetwork(n) && nc.ingressNetwork != nil && nc.ingressNetwork.ID == n.ID {
			nc.ingressNetwork = nil
			if err := a.deallocateNodes(ctx); err != nil {
				log.G(ctx).WithError(err).Error(err)
			}
		}

		// The assumption here is that all dependent objects
		// have been cleaned up when we are here so the only
		// thing that needs to happen is free the network
		// resources.
		if err := nc.nwkAllocator.Deallocate(n); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed during network free for network %s", n.ID)
		}

		delete(nc.unallocatedNetworks, n.ID)
	case api.EventCreateService:
		var s *api.Service
		a.store.View(func(tx store.ReadTx) {
			s = store.GetService(tx, v.Service.ID)
		})

		if s == nil {
			break
		}

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
	case api.EventUpdateService:
		// We may have already allocated this service. If a create or
		// update event is older than the current version in the store,
		// we run the risk of allocating the service a second time.
		// Only operate on the latest version of the service.
		var s *api.Service
		a.store.View(func(tx store.ReadTx) {
			s = store.GetService(tx, v.Service.ID)
		})

		if s == nil {
			break
		}

		if nc.nwkAllocator.IsServiceAllocated(s) {
			if nc.nwkAllocator.PortsAllocatedInHostPublishMode(s) {
				break
			}
			updatePortsInHostPublishMode(s)
		} else {
			if err := a.allocateService(ctx, s); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed allocation during update of service %s", s.ID)
				break
			}
		}

		if _, err := a.store.Batch(func(batch *store.Batch) error {
			return a.commitAllocatedService(ctx, batch, s)
		}); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed to commit allocation during update for service %s", s.ID)
			nc.unallocatedServices[s.ID] = s
		} else {
			delete(nc.unallocatedServices, s.ID)
		}
	case api.EventDeleteService:
		s := v.Service.Copy()

		if err := nc.nwkAllocator.ServiceDeallocate(s); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed deallocation during delete of service %s", s.ID)
		}

		// Remove it from unallocatedServices just in case
		// it's still there.
		delete(nc.unallocatedServices, s.ID)
	case api.EventCreateNode, api.EventUpdateNode, api.EventDeleteNode:
		a.doNodeAlloc(ctx, ev)
	case api.EventCreateTask, api.EventUpdateTask, api.EventDeleteTask:
		a.doTaskAlloc(ctx, ev)
	case state.EventCommit:
		a.procTasksNetwork(ctx, false)

		if time.Since(nc.lastRetry) > retryInterval {
			a.procUnallocatedNetworks(ctx)
			a.procUnallocatedServices(ctx)
			a.procTasksNetwork(ctx, true)
			nc.lastRetry = time.Now()
		}

		// Any left over tasks are moved to the unallocated set
		for _, t := range nc.pendingTasks {
			nc.unallocatedTasks[t.ID] = t
		}
		nc.pendingTasks = make(map[string]*api.Task)
	}
}

func (a *Allocator) doNodeAlloc(ctx context.Context, ev events.Event) {
	var (
		isDelete bool
		node     *api.Node
	)

	// We may have already allocated this node. If a create or update
	// event is older than the current version in the store, we run the
	// risk of allocating the node a second time. Only operate on the
	// latest version of the node.
	switch v := ev.(type) {
	case api.EventCreateNode:
		a.store.View(func(tx store.ReadTx) {
			node = store.GetNode(tx, v.Node.ID)
		})
	case api.EventUpdateNode:
		a.store.View(func(tx store.ReadTx) {
			node = store.GetNode(tx, v.Node.ID)
		})
	case api.EventDeleteNode:
		isDelete = true
		node = v.Node.Copy()
	}

	if node == nil {
		return
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

	if !nc.nwkAllocator.IsNodeAllocated(node) && nc.ingressNetwork != nil {
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

func (a *Allocator) allocateNodes(ctx context.Context) error {
	// Allocate nodes in the store so far before we process watched events.
	var (
		allocatedNodes []*api.Node
		nodes          []*api.Node
		err            error
		nc             = a.netCtx
	)

	a.store.View(func(tx store.ReadTx) {
		nodes, err = store.FindNodes(tx, store.All)
	})
	if err != nil {
		return errors.Wrap(err, "error listing all nodes in store while trying to allocate network resources")
	}

	for _, node := range nodes {
		if nc.nwkAllocator.IsNodeAllocated(node) {
			continue
		}

		if node.Attachment == nil {
			node.Attachment = &api.NetworkAttachment{}
		}

		node.Attachment.Network = nc.ingressNetwork.Copy()
		if err := a.allocateNode(ctx, node); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed to allocate network resources for node %s", node.ID)
			continue
		}

		allocatedNodes = append(allocatedNodes, node)
	}

	if _, err := a.store.Batch(func(batch *store.Batch) error {
		for _, node := range allocatedNodes {
			if err := a.commitAllocatedNode(ctx, batch, node); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to commit allocation of network resources for node %s", node.ID)
			}
		}
		return nil
	}); err != nil {
		log.G(ctx).WithError(err).Error("Failed to commit allocation of network resources for nodes")
	}

	return nil
}

func (a *Allocator) deallocateNodes(ctx context.Context) error {
	var (
		nodes []*api.Node
		nc    = a.netCtx
		err   error
	)

	a.store.View(func(tx store.ReadTx) {
		nodes, err = store.FindNodes(tx, store.All)
	})
	if err != nil {
		return fmt.Errorf("error listing all nodes in store while trying to free network resources")
	}

	for _, node := range nodes {
		if nc.nwkAllocator.IsNodeAllocated(node) {
			if err := nc.nwkAllocator.DeallocateNode(node); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed freeing network resources for node %s", node.ID)
			}
			node.Attachment = nil
			if _, err := a.store.Batch(func(batch *store.Batch) error {
				return a.commitAllocatedNode(ctx, batch, node)
			}); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to commit deallocation of network resources for node %s", node.ID)
			}
		}
	}

	return nil
}

// taskReadyForNetworkVote checks if the task is ready for a network
// vote to move it to PENDING state.
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

// IsIngressNetworkNeeded checks whether the service requires the routing-mesh
func IsIngressNetworkNeeded(s *api.Service) bool {
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
	if IsIngressNetworkNeeded(s) && a.netCtx.ingressNetwork != nil {
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

	// We may have already allocated this task. If a create or update
	// event is older than the current version in the store, we run the
	// risk of allocating the task a second time. Only operate on the
	// latest version of the task.
	switch v := ev.(type) {
	case api.EventCreateTask:
		a.store.View(func(tx store.ReadTx) {
			t = store.GetTask(tx, v.Task.ID)
		})
	case api.EventUpdateTask:
		a.store.View(func(tx store.ReadTx) {
			t = store.GetTask(tx, v.Task.ID)
		})
	case api.EventDeleteTask:
		isDelete = true
		t = v.Task.Copy()
	}

	if t == nil {
		return
	}

	nc := a.netCtx

	// If the task has stopped running then we should free the network
	// resources associated with the task right away.
	if t.Status.State > api.TaskStateRunning || isDelete {
		if nc.nwkAllocator.IsTaskAllocated(t) {
			if err := nc.nwkAllocator.DeallocateTask(t); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed freeing network resources for task %s", t.ID)
			}
		}

		// Cleanup any task references that might exist
		delete(nc.pendingTasks, t.ID)
		delete(nc.unallocatedTasks, t.ID)
		return
	}

	// If we are already in allocated state, there is
	// absolutely nothing else to do.
	if t.Status.State >= api.TaskStatePending {
		delete(nc.pendingTasks, t.ID)
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
			if t.Status.State <= api.TaskStateRunning && !isDelete {
				log.G(ctx).Errorf("Event %T: Failed to get service %s for task %s state %s: could not find service %s", ev, t.ServiceID, t.ID, t.Status.State, t.ServiceID)
				return
			}
		}
	}

	// Populate network attachments in the task
	// based on service spec.
	a.taskCreateNetworkAttachments(t, s)

	nc.pendingTasks[t.ID] = t
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

// This function prepares the service object for being updated when the change regards
// the published ports in host mode: It resets the runtime state ports (s.Endpoint.Ports)
// to the current ingress mode runtime state ports plus the newly configured publish mode ports,
// so that the service allocation invoked on this new service object will trigger the deallocation
// of any old publish mode port and allocation of any new one.
func updatePortsInHostPublishMode(s *api.Service) {
	if s.Endpoint != nil {
		var portConfigs []*api.PortConfig
		for _, portConfig := range s.Endpoint.Ports {
			if portConfig.PublishMode == api.PublishModeIngress {
				portConfigs = append(portConfigs, portConfig)
			}
		}
		s.Endpoint.Ports = portConfigs
	}

	if s.Spec.Endpoint != nil {
		if s.Endpoint == nil {
			s.Endpoint = &api.Endpoint{}
		}
		for _, portConfig := range s.Spec.Endpoint.Ports {
			if portConfig.PublishMode == api.PublishModeIngress {
				continue
			}
			s.Endpoint.Ports = append(s.Endpoint.Ports, portConfig.Copy())
		}
		s.Endpoint.Spec = s.Spec.Endpoint.Copy()
	}
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
		if IsIngressNetworkNeeded(s) {
			if nc.ingressNetwork == nil {
				return fmt.Errorf("ingress network is missing")
			}
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
	if !IsIngressNetworkNeeded(s) && nc.ingressNetwork != nil {
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
				err = errors.Wrapf(err, "failed during network allocation for task %s", t.ID)
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

func (a *Allocator) procTasksNetwork(ctx context.Context, onRetry bool) {
	nc := a.netCtx
	quiet := false
	toAllocate := nc.pendingTasks
	if onRetry {
		toAllocate = nc.unallocatedTasks
		quiet = true
	}
	allocatedTasks := make([]*api.Task, 0, len(toAllocate))

	for _, t := range toAllocate {
		if err := a.allocateTask(ctx, t); err == nil {
			allocatedTasks = append(allocatedTasks, t)
		} else if err != errNoChanges {
			if quiet {
				log.G(ctx).WithError(err).Debug("task allocation failure")
			} else {
				log.G(ctx).WithError(err).Error("task allocation failure")
			}
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
		log.G(ctx).WithError(err).Error("failed a store batch operation while processing tasks")
	}

	for _, t := range allocatedTasks[:committed] {
		delete(toAllocate, t.ID)
	}
}

// updateTaskStatus sets TaskStatus and updates timestamp.
func updateTaskStatus(t *api.Task, newStatus api.TaskState, message string) {
	t.Status.State = newStatus
	t.Status.Message = message
	t.Status.Timestamp = ptypes.MustTimestampProto(time.Now())
}

// IsIngressNetwork returns whether the passed network is an ingress network.
func IsIngressNetwork(nw *api.Network) bool {
	if nw.Spec.Ingress {
		return true
	}
	// Check if legacy defined ingress network
	_, ok := nw.Spec.Annotations.Labels["com.docker.swarm.internal"]
	return ok && nw.Spec.Annotations.Name == "ingress"
}

// GetIngressNetwork fetches the ingress network from store.
// ErrNoIngress will be returned if the ingress network is not present,
// nil otherwise. In case of any other failure in accessing the store,
// the respective error will be reported as is.
func GetIngressNetwork(s *store.MemoryStore) (*api.Network, error) {
	var (
		networks []*api.Network
		err      error
	)
	s.View(func(tx store.ReadTx) {
		networks, err = store.FindNetworks(tx, store.All)
	})
	if err != nil {
		return nil, err
	}
	for _, n := range networks {
		if IsIngressNetwork(n) {
			return n, nil
		}
	}
	return nil, ErrNoIngress
}
