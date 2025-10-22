package daemon

import (
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"gotest.tools/v3/assert"
)

// ServiceConstructor defines a swarm service constructor function
type ServiceConstructor func(*swarm.Service)

func (d *Daemon) createServiceWithOptions(ctx context.Context, t testing.TB, opts client.ServiceCreateOptions, f ...ServiceConstructor) string {
	t.Helper()
	var service swarm.Service
	for _, fn := range f {
		fn(&service)
	}

	cli := d.NewClientT(t)
	defer cli.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	res, err := cli.ServiceCreate(ctx, service.Spec, opts)
	assert.NilError(t, err)
	return res.ID
}

// CreateService creates a swarm service given the specified service constructor
func (d *Daemon) CreateService(ctx context.Context, t testing.TB, f ...ServiceConstructor) string {
	t.Helper()
	return d.createServiceWithOptions(ctx, t, client.ServiceCreateOptions{}, f...)
}

// GetService returns the swarm service corresponding to the specified id
func (d *Daemon) GetService(ctx context.Context, t testing.TB, id string) *swarm.Service {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	res, err := cli.ServiceInspect(ctx, id, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	return &res.Service
}

// GetServiceTasks returns the swarm tasks for the specified service
func (d *Daemon) GetServiceTasks(ctx context.Context, t testing.TB, service string) []swarm.Task {
	return d.GetServiceTasksWithFilters(ctx, t, service, nil)
}

// GetServiceTasksWithFilters returns the swarm tasks for the specified service with additional filters
func (d *Daemon) GetServiceTasksWithFilters(ctx context.Context, t testing.TB, service string, additionalFilters client.Filters) []swarm.Task {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	filterArgs := make(client.Filters).Add("desired-state", "running").Add("service", service)
	for term, values := range additionalFilters {
		filterArgs.Add(term, slices.Collect(maps.Keys(values))...)
	}

	options := client.TaskListOptions{
		Filters: filterArgs,
	}

	taskList, err := cli.TaskList(ctx, options)
	assert.NilError(t, err)
	return taskList.Items
}

// UpdateService updates a swarm service with the specified service constructor
func (d *Daemon) UpdateService(ctx context.Context, t testing.TB, service *swarm.Service, f ...ServiceConstructor) {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	for _, fn := range f {
		fn(service)
	}

	_, err := cli.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, client.ServiceUpdateOptions{})
	assert.NilError(t, err)
}

// RemoveService removes the specified service
func (d *Daemon) RemoveService(ctx context.Context, t testing.TB, id string) {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	_, err := cli.ServiceRemove(ctx, id, client.ServiceRemoveOptions{})
	assert.NilError(t, err)
}

// ListServices returns the list of the current swarm services
func (d *Daemon) ListServices(ctx context.Context, t testing.TB) []swarm.Service {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	res, err := cli.ServiceList(ctx, client.ServiceListOptions{})
	assert.NilError(t, err)
	return res.Items
}

// GetTask returns the swarm task identified by the specified id
func (d *Daemon) GetTask(ctx context.Context, t testing.TB, id string) swarm.Task {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	result, err := cli.TaskInspect(ctx, id, client.TaskInspectOptions{})
	assert.NilError(t, err)
	return result.Task
}
