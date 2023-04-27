package service // import "github.com/docker/docker/integration/service"

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/integration/internal/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// TestServiceListWithStatuses tests that performing a ServiceList operation
// correctly uses the Status parameter, and that the resulting response
// contains correct service statuses.
//
// NOTE(dperny): because it's a pain to elicit the behavior of an unconverged
// service reliably, I'm not testing that an unconverged service returns X
// running and Y desired tasks. Instead, I'm just going to trust that I can
// successfully assign a value to another value without screwing it up. The
// logic for computing service statuses is in swarmkit anyway, not in the
// engine, and is well-tested there, so this test just needs to make sure that
// statuses get correctly associated with the right services.
func TestServiceListWithStatuses(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	// statuses were added in API version 1.41
	skip.If(t, versions.LessThan(testEnv.DaemonInfo.ServerVersion, "1.41"))
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	ctx := context.Background()

	serviceCount := 3
	// create some services.
	for i := 0; i < serviceCount; i++ {
		spec := fullSwarmServiceSpec(fmt.Sprintf("test-list-%d", i), uint64(i+1))
		// for whatever reason, the args "-u root", when included, cause these
		// tasks to fail and exit. instead, we'll just pass no args, which
		// works.
		spec.TaskTemplate.ContainerSpec.Args = []string{}
		resp, err := client.ServiceCreate(ctx, spec, types.ServiceCreateOptions{
			QueryRegistry: false,
		})
		assert.NilError(t, err)
		id := resp.ID
		// we need to wait specifically for the tasks to be running, which the
		// serviceContainerCount function does not do. instead, we'll use a
		// bespoke closure right here.
		poll.WaitOn(t, func(log poll.LogT) poll.Result {
			tasks, err := client.TaskList(context.Background(), types.TaskListOptions{
				Filters: filters.NewArgs(filters.Arg("service", id)),
			})

			running := 0
			for _, task := range tasks {
				if task.Status.State == swarmtypes.TaskStateRunning {
					running++
				}
			}

			switch {
			case err != nil:
				return poll.Error(err)
			case running == i+1:
				return poll.Success()
			default:
				return poll.Continue(
					"running task count %d (%d total), waiting for %d",
					running, len(tasks), i+1,
				)
			}
		})
	}

	// now, let's do the list operation with no status arg set.
	resp, err := client.ServiceList(ctx, types.ServiceListOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Len(resp, serviceCount))
	for _, service := range resp {
		assert.Check(t, is.Nil(service.ServiceStatus))
	}

	// now try again, but with Status: true. This time, we should have statuses
	resp, err = client.ServiceList(ctx, types.ServiceListOptions{Status: true})
	assert.NilError(t, err)
	assert.Check(t, is.Len(resp, serviceCount))
	for _, service := range resp {
		replicas := *service.Spec.Mode.Replicated.Replicas

		assert.Assert(t, service.ServiceStatus != nil)
		// Use assert.Check to not fail out of the test if this fails
		assert.Check(t, is.Equal(service.ServiceStatus.DesiredTasks, replicas))
		assert.Check(t, is.Equal(service.ServiceStatus.RunningTasks, replicas))
	}
}
