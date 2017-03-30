package manager

import (
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state/raft"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
)

const roleReconcileInterval = 5 * time.Second

// roleManager reconciles the raft member list with desired role changes.
type roleManager struct {
	ctx    context.Context
	cancel func()

	store    *store.MemoryStore
	raft     *raft.Node
	doneChan chan struct{}

	// pending contains changed nodes that have not yet been reconciled in
	// the raft member list.
	pending map[string]*api.Node
}

// newRoleManager creates a new roleManager.
func newRoleManager(store *store.MemoryStore, raftNode *raft.Node) *roleManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &roleManager{
		ctx:      ctx,
		cancel:   cancel,
		store:    store,
		raft:     raftNode,
		doneChan: make(chan struct{}),
		pending:  make(map[string]*api.Node),
	}
}

// Run is roleManager's main loop.
// ctx is only used for logging.
func (rm *roleManager) Run(ctx context.Context) {
	defer close(rm.doneChan)

	var (
		nodes    []*api.Node
		ticker   *time.Ticker
		tickerCh <-chan time.Time
	)

	watcher, cancelWatch, err := store.ViewAndWatch(rm.store,
		func(readTx store.ReadTx) error {
			var err error
			nodes, err = store.FindNodes(readTx, store.All)
			return err
		},
		api.EventUpdateNode{})
	defer cancelWatch()

	if err != nil {
		log.G(ctx).WithError(err).Error("failed to check nodes for role changes")
	} else {
		for _, node := range nodes {
			rm.pending[node.ID] = node
			rm.reconcileRole(ctx, node)
		}
		if len(rm.pending) != 0 {
			ticker = time.NewTicker(roleReconcileInterval)
			tickerCh = ticker.C
		}
	}

	for {
		select {
		case event := <-watcher:
			node := event.(api.EventUpdateNode).Node
			rm.pending[node.ID] = node
			rm.reconcileRole(ctx, node)
			if len(rm.pending) != 0 && ticker == nil {
				ticker = time.NewTicker(roleReconcileInterval)
				tickerCh = ticker.C
			}
		case <-tickerCh:
			for _, node := range rm.pending {
				rm.reconcileRole(ctx, node)
			}
			if len(rm.pending) == 0 {
				ticker.Stop()
				ticker = nil
				tickerCh = nil
			}
		case <-rm.ctx.Done():
			if ticker != nil {
				ticker.Stop()
			}
			return
		}
	}
}

func (rm *roleManager) reconcileRole(ctx context.Context, node *api.Node) {
	if node.Role == node.Spec.DesiredRole {
		// Nothing to do.
		delete(rm.pending, node.ID)
		return
	}

	// Promotion can proceed right away.
	if node.Spec.DesiredRole == api.NodeRoleManager && node.Role == api.NodeRoleWorker {
		err := rm.store.Update(func(tx store.Tx) error {
			updatedNode := store.GetNode(tx, node.ID)
			if updatedNode == nil || updatedNode.Spec.DesiredRole != node.Spec.DesiredRole || updatedNode.Role != node.Role {
				return nil
			}
			updatedNode.Role = api.NodeRoleManager
			return store.UpdateNode(tx, updatedNode)
		})
		if err != nil {
			log.G(ctx).WithError(err).Errorf("failed to promote node %s", node.ID)
		} else {
			delete(rm.pending, node.ID)
		}
	} else if node.Spec.DesiredRole == api.NodeRoleWorker && node.Role == api.NodeRoleManager {
		// Check for node in memberlist
		member := rm.raft.GetMemberByNodeID(node.ID)
		if member != nil {
			// Quorum safeguard
			if !rm.raft.CanRemoveMember(member.RaftID) {
				// TODO(aaronl): Retry later
				log.G(ctx).Debugf("can't demote node %s at this time: removing member from raft would result in a loss of quorum", node.ID)
				return
			}

			rmCtx, rmCancel := context.WithTimeout(rm.ctx, 5*time.Second)
			defer rmCancel()

			if member.RaftID == rm.raft.Config.ID {
				// Don't use rmCtx, because we expect to lose
				// leadership, which will cancel this context.
				log.G(ctx).Info("demoted; transferring leadership")
				err := rm.raft.TransferLeadership(context.Background())
				if err == nil {
					return
				}
				log.G(ctx).WithError(err).Info("failed to transfer leadership")
			}
			if err := rm.raft.RemoveMember(rmCtx, member.RaftID); err != nil {
				// TODO(aaronl): Retry later
				log.G(ctx).WithError(err).Debugf("can't demote node %s at this time", node.ID)
			}
			return
		}

		err := rm.store.Update(func(tx store.Tx) error {
			updatedNode := store.GetNode(tx, node.ID)
			if updatedNode == nil || updatedNode.Spec.DesiredRole != node.Spec.DesiredRole || updatedNode.Role != node.Role {
				return nil
			}
			updatedNode.Role = api.NodeRoleWorker

			return store.UpdateNode(tx, updatedNode)
		})
		if err != nil {
			log.G(ctx).WithError(err).Errorf("failed to demote node %s", node.ID)
		} else {
			delete(rm.pending, node.ID)
		}
	}
}

// Stop stops the roleManager and waits for the main loop to exit.
func (rm *roleManager) Stop() {
	rm.cancel()
	<-rm.doneChan
}
