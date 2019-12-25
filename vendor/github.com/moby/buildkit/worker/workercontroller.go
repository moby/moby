package worker

import (
	"sync"

	"github.com/containerd/containerd/filters"
	"github.com/moby/buildkit/client"
	"github.com/pkg/errors"
)

// Controller holds worker instances.
// Currently, only local workers are supported.
type Controller struct {
	// TODO: define worker interface and support remote ones
	workers   sync.Map
	defaultID string
}

// Add adds a local worker
func (c *Controller) Add(w Worker) error {
	c.workers.Store(w.ID(), w)
	if c.defaultID == "" {
		c.defaultID = w.ID()
	}
	return nil
}

// List lists workers
func (c *Controller) List(filterStrings ...string) ([]Worker, error) {
	filter, err := filters.ParseAll(filterStrings...)
	if err != nil {
		return nil, err
	}
	var workers []Worker
	c.workers.Range(func(k, v interface{}) bool {
		w := v.(Worker)
		if filter.Match(adaptWorker(w)) {
			workers = append(workers, w)
		}
		return true
	})
	return workers, nil
}

// GetDefault returns the default local worker
func (c *Controller) GetDefault() (Worker, error) {
	if c.defaultID == "" {
		return nil, errors.Errorf("no default worker")
	}
	return c.Get(c.defaultID)
}

func (c *Controller) Get(id string) (Worker, error) {
	v, ok := c.workers.Load(id)
	if !ok {
		return nil, errors.Errorf("worker %s not found", id)
	}
	return v.(Worker), nil
}

// TODO: add Get(Constraint) (*Worker, error)

func (c *Controller) WorkerInfos() []client.WorkerInfo {
	workers, err := c.List()
	if err != nil {
		return nil
	}
	out := make([]client.WorkerInfo, 0, len(workers))
	for _, w := range workers {
		out = append(out, client.WorkerInfo{
			ID:        w.ID(),
			Labels:    w.Labels(),
			Platforms: w.Platforms(),
		})
	}
	return out
}
