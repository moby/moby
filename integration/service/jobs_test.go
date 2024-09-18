package service

import (
	"testing"

	"github.com/docker/docker/api/types"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration/internal/swarm"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// The file jobs_test.go contains tests that verify that services which are in
// the mode ReplicatedJob or GlobalJob.

// TestCreateJob tests that a Service can be created and run with
// mode ReplicatedJob
func TestCreateJob(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)

	client := d.NewClientT(t)
	defer client.Close()

	for _, mode := range []swarmtypes.ServiceMode{
		{ReplicatedJob: &swarmtypes.ReplicatedJob{}},
		{GlobalJob: &swarmtypes.GlobalJob{}},
	} {
		id := swarm.CreateService(ctx, t, d, swarm.ServiceWithMode(mode))

		poll.WaitOn(t, swarm.RunningTasksCount(ctx, client, id, 1), swarm.ServicePoll)
	}
}

// TestReplicatedJob tests that running a replicated job starts the requisite
// number of tasks,
func TestReplicatedJob(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	// we need variables, because the replicas field takes a pointer
	maxConcurrent := uint64(2)
	// there is overhead, especially in the test environment, associated with
	// starting tasks. if total is set too high, then the time needed to
	// complete the test, even if everything is proceeding ideally, may exceed
	// the time the test has to execute
	//
	// in CI,the test has been seen to time out with as few as 7 completions
	// after 15 seconds. this means 7 completions ought not be too many.
	total := uint64(7)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)

	client := d.NewClientT(t)
	defer client.Close()

	id := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithMode(swarmtypes.ServiceMode{
			ReplicatedJob: &swarmtypes.ReplicatedJob{
				MaxConcurrent:    &maxConcurrent,
				TotalCompletions: &total,
			},
		}),
		// just run a command to execute and exit peacefully.
		swarm.ServiceWithCommand([]string{"true"}),
	)

	service, _, err := client.ServiceInspectWithRaw(
		ctx, id, types.ServiceInspectOptions{},
	)
	assert.NilError(t, err)

	poll.WaitOn(t, swarm.JobComplete(ctx, client, service), swarm.ServicePoll)
}

// TestUpdateJob tests that a job can be updated, and that it runs with the
// correct parameters.
func TestUpdateReplicatedJob(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)

	client := d.NewClientT(t)
	defer client.Close()

	// Create the job service
	id := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithMode(swarmtypes.ServiceMode{
			ReplicatedJob: &swarmtypes.ReplicatedJob{
				// use the default, empty values.
			},
		}),
		// run "true" so the task exits with 0
		swarm.ServiceWithCommand([]string{"true"}),
	)

	service, _, err := client.ServiceInspectWithRaw(
		ctx, id, types.ServiceInspectOptions{},
	)
	assert.NilError(t, err)

	// wait for the job to completed
	poll.WaitOn(t, swarm.JobComplete(ctx, client, service), swarm.ServicePoll)

	// update the job.
	spec := service.Spec
	spec.TaskTemplate.ForceUpdate++

	_, err = client.ServiceUpdate(
		ctx, id, service.Version, spec, types.ServiceUpdateOptions{},
	)
	assert.NilError(t, err)

	service2, _, err := client.ServiceInspectWithRaw(
		ctx, id, types.ServiceInspectOptions{},
	)
	assert.NilError(t, err)

	// assert that the job iteration has increased
	assert.Assert(t,
		service.JobStatus.JobIteration.Index < service2.JobStatus.JobIteration.Index,
	)

	// now wait for the service to complete a second time.
	poll.WaitOn(t, swarm.JobComplete(ctx, client, service2), swarm.ServicePoll)
}
