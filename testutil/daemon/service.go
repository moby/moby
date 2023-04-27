package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"gotest.tools/v3/assert"
)

// ServiceConstructor defines a swarm service constructor function
type ServiceConstructor func(*swarm.Service)

func (d *Daemon) createServiceWithOptions(t testing.TB, opts types.ServiceCreateOptions, f ...ServiceConstructor) string {
	t.Helper()
	var service swarm.Service
	for _, fn := range f {
		fn(&service)
	}

	cli := d.NewClientT(t)
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := cli.ServiceCreate(ctx, service.Spec, opts)
	assert.NilError(t, err)
	return res.ID
}

// CreateService creates a swarm service given the specified service constructor
func (d *Daemon) CreateService(t testing.TB, f ...ServiceConstructor) string {
	t.Helper()
	return d.createServiceWithOptions(t, types.ServiceCreateOptions{}, f...)
}

// GetService returns the swarm service corresponding to the specified id
func (d *Daemon) GetService(t testing.TB, id string) *swarm.Service {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	service, _, err := cli.ServiceInspectWithRaw(context.Background(), id, types.ServiceInspectOptions{})
	assert.NilError(t, err)
	return &service
}

// GetServiceTasks returns the swarm tasks for the specified service
func (d *Daemon) GetServiceTasks(t testing.TB, service string, additionalFilters ...filters.KeyValuePair) []swarm.Task {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	filterArgs := filters.NewArgs(
		filters.Arg("desired-state", "running"),
		filters.Arg("service", service),
	)
	for _, filter := range additionalFilters {
		filterArgs.Add(filter.Key, filter.Value)
	}

	options := types.TaskListOptions{
		Filters: filterArgs,
	}

	tasks, err := cli.TaskList(context.Background(), options)
	assert.NilError(t, err)
	return tasks
}

// UpdateService updates a swarm service with the specified service constructor
func (d *Daemon) UpdateService(t testing.TB, service *swarm.Service, f ...ServiceConstructor) {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	for _, fn := range f {
		fn(service)
	}

	_, err := cli.ServiceUpdate(context.Background(), service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
}

// RemoveService removes the specified service
func (d *Daemon) RemoveService(t testing.TB, id string) {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	err := cli.ServiceRemove(context.Background(), id)
	assert.NilError(t, err)
}

// ListServices returns the list of the current swarm services
func (d *Daemon) ListServices(t testing.TB) []swarm.Service {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	services, err := cli.ServiceList(context.Background(), types.ServiceListOptions{})
	assert.NilError(t, err)
	return services
}

// GetTask returns the swarm task identified by the specified id
func (d *Daemon) GetTask(t testing.TB, id string) swarm.Task {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	task, _, err := cli.TaskInspectWithRaw(context.Background(), id)
	assert.NilError(t, err)
	return task
}
