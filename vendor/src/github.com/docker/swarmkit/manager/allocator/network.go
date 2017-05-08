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
	"golang.org/x/net/context"
)

const (
	// Network allocator Voter ID for task allocation vote.
	networkVoter = "network"

	ingressNetworkName = "ingress"
	ingressSubnet      = "10.255.0.0/16"
)

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
	na, err := networkallocator.New()
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
		return fmt.Errorf("failed to find ingress network during init: %v", err)
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
			return fmt.Errorf("failed to create ingress network: %v", err)
		}

		a.store.View(func(tx store.ReadTx) {
			networks, err = store.FindNetworks(tx, store.ByName(ingressNetworkName))
			if len(networks) > 0 {
				nc.ingressNetwork = networks[0]
			}
		})
		if err != nil {
			return fmt.Errorf("failed to find ingress network after creating it: %v", err)
		}

	}

	// Try to complete ingress network allocation before anything else so
	// that the we can get the preferred subnet for ingress
	// network.
	if !na.IsAllocated(nc.ingressNetwork) {
		if err := a.allocateNetwork(ctx, nc.ingressNetwork); err != nil {
			log.G(ctx).Errorf("failed allocating ingress network during init: %v", err)
		}

		// Update store after allocation
		if err := a.store.Update(func(tx store.Tx) error {
			if err := store.UpdateNetwork(tx, nc.ingressNetwork); err != nil {
				return err
			}

			return nil
		}); err != nil {
			return fmt.Errorf("failed to create ingress network: %v", err)
		}
	}

	// Allocate networks in the store so far before we started
	// watching.
	a.store.View(func(tx store.ReadTx) {
		networks, err = store.FindNetworks(tx, store.All)
	})
	if err != nil {
		return fmt.Errorf("error listing all networks in store while trying to allocate during init: %v", err)
	}

	for _, n := range networks {
		if na.IsAllocated(n) {
			continue
		}

		if err := a.allocateNetwork(ctx, n); err != nil {
			log.G(ctx).Errorf("failed allocating network %s during init: %v", n.ID, err)
		}
	}

	// Allocate nodes in the store so far before we process watched events.
	var nodes []*api.Node
	a.store.View(func(tx store.ReadTx) {
		nodes, err = store.FindNodes(tx, store.All)
	})
	if err != nil {
		return fmt.Errorf("error listing all services in store while trying to allocate during init: %v", err)
	}

	for _, node := range nodes {
		if na.IsNodeAllocated(node) {
			continue
		}

		if node.Attachment == nil {
			node.Attachment = &api.NetworkAttachment{}
		}

		node.Attachment.Network = nc.ingressNetwork.Copy()
		if err := a.allocateNode(ctx, node); err != nil {
			log.G(ctx).Errorf("Failed to allocate network resources for node %s during init: %v", node.ID, err)
		}
	}

	// Allocate services in the store so far before we process watched events.
	var services []*api.Service
	a.store.View(func(tx store.ReadTx) {
		services, err = store.FindServices(tx, store.All)
	})
	if err != nil {
		return fmt.Errorf("error listing all services in store while trying to allocate during init: %v", err)
	}

	for _, s := range services {
		if nc.nwkAllocator.IsServiceAllocated(s) {
			continue
		}

		if err := a.allocateService(ctx, s); err != nil {
			log.G(ctx).Errorf("failed allocating service %s during init: %v", s.ID, err)
		}
	}

	// Allocate tasks in the store so far before we started watching.
	var tasks []*api.Task
	a.store.View(func(tx store.ReadTx) {
		tasks, err = store.FindTasks(tx, store.All)
	})
	if err != nil {
		return fmt.Errorf("error listing all tasks in store while trying to allocate during init: %v", err)
	}

	if _, err := a.store.Batch(func(batch *store.Batch) error {
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
				if t.Status.State >= api.TaskStateAllocated {
					continue
				}

				if a.taskAllocateVote(networkVoter, t.ID) {
					// If the task is not attached to any network, network
					// allocators job is done. Immediately cast a vote so
					// that the task can be moved to ALLOCATED state as
					// soon as possible.
					if err := batch.Update(func(tx store.Tx) error {
						storeT := store.GetTask(tx, t.ID)
						if storeT == nil {
							return fmt.Errorf("task %s not found while trying to update state", t.ID)
						}

						updateTaskStatus(storeT, api.TaskStateAllocated, "allocated")

						if err := store.UpdateTask(tx, storeT); err != nil {
							return fmt.Errorf("failed updating state in store transaction for task %s: %v", storeT.ID, err)
						}

						return nil
					}); err != nil {
						log.G(ctx).WithError(err).Error("error updating task network")
					}
				}
				continue
			}

			err := batch.Update(func(tx store.Tx) error {
				_, err := a.allocateTask(ctx, tx, t)
				return err
			})
			if err != nil {
				log.G(ctx).Errorf("failed allocating task %s during init: %v", t.ID, err)
				nc.unallocatedTasks[t.ID] = t
			}
		}

		return nil
	}); err != nil {
		return err
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
			log.G(ctx).Errorf("Failed allocation for network %s: %v", n.ID, err)
			break
		}
	case state.EventDeleteNetwork:
		n := v.Network.Copy()

		// The assumption here is that all dependent objects
		// have been cleaned up when we are here so the only
		// thing that needs to happen is free the network
		// resources.
		if err := nc.nwkAllocator.Deallocate(n); err != nil {
			log.G(ctx).Errorf("Failed during network free for network %s: %v", n.ID, err)
		}
	case state.EventCreateService:
		s := v.Service.Copy()

		if nc.nwkAllocator.IsServiceAllocated(s) {
			break
		}

		if err := a.allocateService(ctx, s); err != nil {
			log.G(ctx).Errorf("Failed allocation for service %s: %v", s.ID, err)
			break
		}
	case state.EventUpdateService:
		s := v.Service.Copy()

		if nc.nwkAllocator.IsServiceAllocated(s) {
			break
		}

		if err := a.allocateService(ctx, s); err != nil {
			log.G(ctx).Errorf("Failed allocation during update of service %s: %v", s.ID, err)
			break
		}
	case state.EventDeleteService:
		s := v.Service.Copy()

		if err := nc.nwkAllocator.ServiceDeallocate(s); err != nil {
			log.G(ctx).Errorf("Failed deallocation during delete of service %s: %v", s.ID, err)
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
				log.G(ctx).Errorf("Failed freeing network resources for node %s: %v", node.ID, err)
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
			log.G(ctx).Errorf("Failed to allocate network resources for node %s: %v", node.ID, err)
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

func (a *Allocator) taskCreateNetworkAttachments(t *api.Task, s *api.Service) {
	// If service is nil or if task network attachments have
	// already been filled in no need to do anything else.
	if s == nil || len(t.Networks) != 0 {
		return
	}

	var networks []*api.NetworkAttachment

	// The service to which this task belongs is trying to expose
	// ports to the external world. Automatically attach the task
	// to the ingress network.
	if s.Spec.Endpoint != nil && len(s.Spec.Endpoint.Ports) != 0 {
		networks = append(networks, &api.NetworkAttachment{Network: a.netCtx.ingressNetwork})
	}

	a.store.View(func(tx store.ReadTx) {
		for _, na := range s.Spec.Networks {
			n := store.GetNetwork(tx, na.Target)
			if n != nil {
				var aliases []string
				for _, a := range na.Aliases {
					aliases = append(aliases, a)
				}
				networks = append(networks, &api.NetworkAttachment{Network: n, Aliases: aliases})
			}
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
				log.G(ctx).Errorf("Failed freeing network resources for task %s: %v", t.ID, err)
			}
		}

		// Cleanup any task references that might exist in unallocatedTasks
		delete(nc.unallocatedTasks, t.ID)
		return
	}

	// If we are already in allocated state, there is
	// absolutely nothing else to do.
	if t.Status.State >= api.TaskStateAllocated {
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

		// Populate network attachments in the task
		// based on service spec.
		a.taskCreateNetworkAttachments(t, s)
	}

	nc.unallocatedTasks[t.ID] = t
}

func (a *Allocator) allocateNode(ctx context.Context, node *api.Node) error {
	nc := a.netCtx

	if err := nc.nwkAllocator.AllocateNode(node); err != nil {
		return err
	}

	if err := a.store.Update(func(tx store.Tx) error {
		for {
			err := store.UpdateNode(tx, node)
			if err != nil && err != store.ErrSequenceConflict {
				return fmt.Errorf("failed updating state in store transaction for node %s: %v", node.ID, err)
			}

			if err == store.ErrSequenceConflict {
				storeNode := store.GetNode(tx, node.ID)
				storeNode.Attachment = node.Attachment.Copy()
				node = storeNode
				continue
			}

			break
		}
		return nil
	}); err != nil {
		if err := nc.nwkAllocator.DeallocateNode(node); err != nil {
			log.G(ctx).WithError(err).Errorf("failed rolling back allocation of node %s: %v", node.ID, err)
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
		if len(s.Spec.Endpoint.Ports) != 0 {
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
	if s.Spec.Endpoint == nil || len(s.Spec.Endpoint.Ports) == 0 {
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

	if err := a.store.Update(func(tx store.Tx) error {
		for {
			err := store.UpdateService(tx, s)

			if err != nil && err != store.ErrSequenceConflict {
				return fmt.Errorf("failed updating state in store transaction for service %s: %v", s.ID, err)
			}

			if err == store.ErrSequenceConflict {
				storeService := store.GetService(tx, s.ID)
				storeService.Endpoint = s.Endpoint
				s = storeService
				continue
			}

			break
		}
		return nil
	}); err != nil {
		if err := nc.nwkAllocator.ServiceDeallocate(s); err != nil {
			log.G(ctx).WithError(err).Errorf("failed rolling back allocation of service %s: %v", s.ID, err)
		}

		return err
	}

	return nil
}

func (a *Allocator) allocateNetwork(ctx context.Context, n *api.Network) error {
	nc := a.netCtx

	if err := nc.nwkAllocator.Allocate(n); err != nil {
		nc.unallocatedNetworks[n.ID] = n
		return fmt.Errorf("failed during network allocation for network %s: %v", n.ID, err)
	}

	if err := a.store.Update(func(tx store.Tx) error {
		if err := store.UpdateNetwork(tx, n); err != nil {
			return fmt.Errorf("failed updating state in store transaction for network %s: %v", n.ID, err)
		}
		return nil
	}); err != nil {
		if err := nc.nwkAllocator.Deallocate(n); err != nil {
			log.G(ctx).WithError(err).Errorf("failed rolling back allocation of network %s", n.ID)
		}

		return err
	}

	return nil
}

func (a *Allocator) allocateTask(ctx context.Context, tx store.Tx, t *api.Task) (*api.Task, error) {
	taskUpdated := false

	// Get the latest task state from the store before updating.
	storeT := store.GetTask(tx, t.ID)
	if storeT == nil {
		return nil, fmt.Errorf("could not find task %s while trying to update network allocation", t.ID)
	}

	nc := a.netCtx

	// We might be here even if a task allocation has already
	// happened but wasn't successfully committed to store. In such
	// cases skip allocation and go straight ahead to updating the
	// store.
	if !nc.nwkAllocator.IsTaskAllocated(t) {
		if t.ServiceID != "" {
			s := store.GetService(tx, t.ServiceID)
			if s == nil {
				return nil, fmt.Errorf("could not find service %s", t.ServiceID)
			}

			if !nc.nwkAllocator.IsServiceAllocated(s) {
				return nil, fmt.Errorf("service %s to which this task %s belongs has pending allocations", s.ID, t.ID)
			}

			taskUpdateEndpoint(t, s.Endpoint)
		}

		for _, na := range t.Networks {
			n := store.GetNetwork(tx, na.Network.ID)
			if n == nil {
				return nil, fmt.Errorf("failed to retrieve network %s while allocating task %s", na.Network.ID, t.ID)
			}

			if !nc.nwkAllocator.IsAllocated(n) {
				return nil, fmt.Errorf("network %s attached to task %s not allocated yet", n.ID, t.ID)
			}

			na.Network = n
		}

		if err := nc.nwkAllocator.AllocateTask(t); err != nil {
			return nil, fmt.Errorf("failed during networktask allocation for task %s: %v", t.ID, err)
		}
		if nc.nwkAllocator.IsTaskAllocated(t) {
			taskUpdateNetworks(storeT, t.Networks)
			taskUpdateEndpoint(storeT, t.Endpoint)
			taskUpdated = true
		}
	}

	// Update the network allocations and moving to
	// ALLOCATED state on top of the latest store state.
	if a.taskAllocateVote(networkVoter, t.ID) {
		if storeT.Status.State < api.TaskStateAllocated {
			updateTaskStatus(storeT, api.TaskStateAllocated, "allocated")
			taskUpdated = true
		}
	}

	if taskUpdated {
		if err := store.UpdateTask(tx, storeT); err != nil {
			return nil, fmt.Errorf("failed updating state in store transaction for task %s: %v", storeT.ID, err)
		}
	}

	return storeT, nil
}

func (a *Allocator) procUnallocatedNetworks(ctx context.Context) {
	nc := a.netCtx
	for _, n := range nc.unallocatedNetworks {
		if !nc.nwkAllocator.IsAllocated(n) {
			if err := a.allocateNetwork(ctx, n); err != nil {
				log.G(ctx).Debugf("Failed allocation of unallocated network %s: %v", n.ID, err)
				continue
			}
		}

		delete(nc.unallocatedNetworks, n.ID)
	}
}

func (a *Allocator) procUnallocatedServices(ctx context.Context) {
	nc := a.netCtx
	for _, s := range nc.unallocatedServices {
		if !nc.nwkAllocator.IsServiceAllocated(s) {
			if err := a.allocateService(ctx, s); err != nil {
				log.G(ctx).Debugf("Failed allocation of unallocated service %s: %v", s.ID, err)
				continue
			}
		}

		delete(nc.unallocatedServices, s.ID)
	}
}

func (a *Allocator) procUnallocatedTasksNetwork(ctx context.Context) {
	nc := a.netCtx
	tasks := make([]*api.Task, 0, len(nc.unallocatedTasks))

	committed, err := a.store.Batch(func(batch *store.Batch) error {
		for _, t := range nc.unallocatedTasks {
			var allocatedT *api.Task
			err := batch.Update(func(tx store.Tx) error {
				var err error
				allocatedT, err = a.allocateTask(ctx, tx, t)
				return err
			})

			if err != nil {
				log.G(ctx).WithError(err).Error("task allocation failure")
				continue
			}

			tasks = append(tasks, allocatedT)
		}

		return nil
	})

	if err != nil {
		log.G(ctx).WithError(err).Error("failed a store batch operation while processing unallocated tasks")
	}

	var retryCnt int
	for len(tasks) != 0 {
		var err error

		for _, t := range tasks[:committed] {
			delete(nc.unallocatedTasks, t.ID)
		}

		tasks = tasks[committed:]
		if len(tasks) == 0 {
			break
		}

		updatedTasks := make([]*api.Task, 0, len(tasks))
		committed, err = a.store.Batch(func(batch *store.Batch) error {
			for _, t := range tasks {
				err := batch.Update(func(tx store.Tx) error {
					return store.UpdateTask(tx, t)
				})

				if err != nil {
					log.G(ctx).WithError(err).Error("allocated task store update failure")
					continue
				}

				updatedTasks = append(updatedTasks, t)
			}

			return nil
		})
		if err != nil {
			log.G(ctx).WithError(err).Error("failed a store batch operation while processing unallocated tasks")
		}

		tasks = updatedTasks

		select {
		case <-ctx.Done():
			return
		default:
		}

		retryCnt++
		if retryCnt >= 3 {
			log.G(ctx).Errorf("failed to complete batch update of allocated tasks after 3 retries")
			break
		}
	}
}

// updateTaskStatus sets TaskStatus and updates timestamp.
func updateTaskStatus(t *api.Task, newStatus api.TaskState, message string) {
	t.Status.State = newStatus
	t.Status.Message = message
	t.Status.Timestamp = ptypes.MustTimestampProto(time.Now())
}
