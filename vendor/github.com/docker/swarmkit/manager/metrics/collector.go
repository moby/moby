package metrics

import (
	"context"

	"strings"

	metrics "github.com/docker/go-metrics"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state/store"
)

var (
	ns          = metrics.NewNamespace("swarm", "manager", nil)
	nodesMetric metrics.LabeledGauge
)

func init() {
	nodesMetric = ns.NewLabeledGauge("nodes", "The number of nodes", "", "state")
	for _, state := range api.NodeStatus_State_name {
		nodesMetric.WithValues(strings.ToLower(state)).Set(0)
	}
	metrics.Register(ns)
}

// Collector collects swarmkit metrics
type Collector struct {
	store *store.MemoryStore

	// stopChan signals to the state machine to stop running.
	stopChan chan struct{}
	// doneChan is closed when the state machine terminates.
	doneChan chan struct{}
}

// NewCollector creates a new metrics collector
func NewCollector(store *store.MemoryStore) *Collector {
	return &Collector{
		store:    store,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
}

func (c *Collector) updateNodeState(prevNode, newNode *api.Node) {
	// Skip updates if nothing changed.
	if prevNode != nil && newNode != nil && prevNode.Status.State == newNode.Status.State {
		return
	}

	if prevNode != nil {
		nodesMetric.WithValues(strings.ToLower(prevNode.Status.State.String())).Dec(1)
	}
	if newNode != nil {
		nodesMetric.WithValues(strings.ToLower(newNode.Status.State.String())).Inc(1)
	}
}

// Run contains the collector event loop
func (c *Collector) Run(ctx context.Context) error {
	defer close(c.doneChan)

	watcher, cancel, err := store.ViewAndWatch(c.store, func(readTx store.ReadTx) error {
		nodes, err := store.FindNodes(readTx, store.All)
		if err != nil {
			return err
		}
		for _, node := range nodes {
			c.updateNodeState(nil, node)
		}
		return nil
	})
	if err != nil {
		return err
	}
	defer cancel()

	for {
		select {
		case event := <-watcher:
			switch v := event.(type) {
			case api.EventCreateNode:
				c.updateNodeState(nil, v.Node)
			case api.EventUpdateNode:
				c.updateNodeState(v.OldNode, v.Node)
			case api.EventDeleteNode:
				c.updateNodeState(v.Node, nil)
			}
		case <-c.stopChan:
			return nil
		}
	}
}

// Stop stops the collector.
func (c *Collector) Stop() {
	close(c.stopChan)
	<-c.doneChan

	// Clean the metrics on exit.
	for _, state := range api.NodeStatus_State_name {
		nodesMetric.WithValues(strings.ToLower(state)).Set(0)
	}
}
