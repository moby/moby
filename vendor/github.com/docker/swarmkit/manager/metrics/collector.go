package metrics

import (
	"context"

	"strings"

	"github.com/docker/go-events"
	metrics "github.com/docker/go-metrics"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state/store"
)

var (
	ns = metrics.NewNamespace("swarm", "manager", nil)

	// counts of the various objects in swarmkit
	nodesMetric metrics.LabeledGauge
	tasksMetric metrics.LabeledGauge

	// none of these objects have state, so they're just regular gauges
	servicesMetric metrics.Gauge
	networksMetric metrics.Gauge
	secretsMetric  metrics.Gauge
	configsMetric  metrics.Gauge
)

func init() {
	nodesMetric = ns.NewLabeledGauge("nodes", "The number of nodes", "", "state")
	tasksMetric = ns.NewLabeledGauge("tasks", "The number of tasks in the cluster object store", metrics.Total, "state")
	servicesMetric = ns.NewGauge("services", "The number of services in the cluster object store", metrics.Total)
	networksMetric = ns.NewGauge("networks", "The number of networks in the cluster object store", metrics.Total)
	secretsMetric = ns.NewGauge("secrets", "The number of secrets in the cluster object store", metrics.Total)
	configsMetric = ns.NewGauge("configs", "The number of configs in the cluster object store", metrics.Total)

	resetMetrics()

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

// Run contains the collector event loop
func (c *Collector) Run(ctx context.Context) error {
	defer close(c.doneChan)

	watcher, cancel, err := store.ViewAndWatch(c.store, func(readTx store.ReadTx) error {
		nodes, err := store.FindNodes(readTx, store.All)
		if err != nil {
			return err
		}
		tasks, err := store.FindTasks(readTx, store.All)
		if err != nil {
			return err
		}
		services, err := store.FindServices(readTx, store.All)
		if err != nil {
			return err
		}
		networks, err := store.FindNetworks(readTx, store.All)
		if err != nil {
			return err
		}
		secrets, err := store.FindSecrets(readTx, store.All)
		if err != nil {
			return err
		}
		configs, err := store.FindConfigs(readTx, store.All)
		if err != nil {
			return err
		}

		for _, obj := range nodes {
			c.handleEvent(obj.EventCreate())
		}
		for _, obj := range tasks {
			c.handleEvent(obj.EventCreate())
		}
		for _, obj := range services {
			c.handleEvent(obj.EventCreate())
		}
		for _, obj := range networks {
			c.handleEvent(obj.EventCreate())
		}
		for _, obj := range secrets {
			c.handleEvent(obj.EventCreate())
		}
		for _, obj := range configs {
			c.handleEvent(obj.EventCreate())
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
			c.handleEvent(event)
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
	resetMetrics()
}

// resetMetrics resets all metrics to their default (base) value
func resetMetrics() {
	for _, state := range api.NodeStatus_State_name {
		nodesMetric.WithValues(strings.ToLower(state)).Set(0)
	}
	for _, state := range api.TaskState_name {
		tasksMetric.WithValues(strings.ToLower(state)).Set(0)
	}
	servicesMetric.Set(0)
	networksMetric.Set(0)
	secretsMetric.Set(0)
	configsMetric.Set(0)

}

// handleEvent handles a single incoming cluster event.
func (c *Collector) handleEvent(event events.Event) {
	switch event.(type) {
	case api.EventNode:
		c.handleNodeEvent(event)
	case api.EventTask:
		c.handleTaskEvent(event)
	case api.EventService:
		c.handleServiceEvent(event)
	case api.EventNetwork:
		c.handleNetworkEvent(event)
	case api.EventSecret:
		c.handleSecretsEvent(event)
	case api.EventConfig:
		c.handleConfigsEvent(event)
	}
}

func (c *Collector) handleNodeEvent(event events.Event) {
	var prevNode, newNode *api.Node

	switch v := event.(type) {
	case api.EventCreateNode:
		prevNode, newNode = nil, v.Node
	case api.EventUpdateNode:
		prevNode, newNode = v.OldNode, v.Node
	case api.EventDeleteNode:
		prevNode, newNode = v.Node, nil
	}

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
	return
}

func (c *Collector) handleTaskEvent(event events.Event) {
	var prevTask, newTask *api.Task

	switch v := event.(type) {
	case api.EventCreateTask:
		prevTask, newTask = nil, v.Task
	case api.EventUpdateTask:
		prevTask, newTask = v.OldTask, v.Task
	case api.EventDeleteTask:
		prevTask, newTask = v.Task, nil
	}

	// Skip updates if nothing changed.
	if prevTask != nil && newTask != nil && prevTask.Status.State == newTask.Status.State {
		return
	}

	if prevTask != nil {
		tasksMetric.WithValues(
			strings.ToLower(prevTask.Status.State.String()),
		).Dec(1)
	}
	if newTask != nil {
		tasksMetric.WithValues(
			strings.ToLower(newTask.Status.State.String()),
		).Inc(1)
	}

	return
}

func (c *Collector) handleServiceEvent(event events.Event) {
	switch event.(type) {
	case api.EventCreateService:
		servicesMetric.Inc(1)
	case api.EventDeleteService:
		servicesMetric.Dec(1)
	}
}

func (c *Collector) handleNetworkEvent(event events.Event) {
	switch event.(type) {
	case api.EventCreateNetwork:
		networksMetric.Inc(1)
	case api.EventDeleteNetwork:
		networksMetric.Dec(1)
	}
}

func (c *Collector) handleSecretsEvent(event events.Event) {
	switch event.(type) {
	case api.EventCreateSecret:
		secretsMetric.Inc(1)
	case api.EventDeleteSecret:
		secretsMetric.Dec(1)
	}
}

func (c *Collector) handleConfigsEvent(event events.Event) {
	switch event.(type) {
	case api.EventCreateConfig:
		configsMetric.Inc(1)
	case api.EventDeleteConfig:
		configsMetric.Dec(1)
	}
}
