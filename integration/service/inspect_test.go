package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/integration-cli/request"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestInspect(t *testing.T) {
	skip.IfCondition(t, !testEnv.IsLocalDaemon())
	defer setupTest(t)()
	d := newSwarm(t)
	defer d.Stop(t)
	client, err := request.NewClientForHost(d.Sock())
	require.NoError(t, err)

	var before = time.Now()
	var instances uint64 = 2
	serviceSpec := fullSwarmServiceSpec("test-service-inspect", instances)

	ctx := context.Background()
	resp, err := client.ServiceCreate(ctx, serviceSpec, types.ServiceCreateOptions{
		QueryRegistry: false,
	})
	require.NoError(t, err)

	id := resp.ID
	poll.WaitOn(t, serviceContainerCount(client, id, instances))

	service, _, err := client.ServiceInspectWithRaw(ctx, id, types.ServiceInspectOptions{})
	require.NoError(t, err)
	assert.Equal(t, serviceSpec, service.Spec)
	assert.Equal(t, uint64(11), service.Meta.Version.Index)
	assert.Equal(t, id, service.ID)
	assert.WithinDuration(t, before, service.CreatedAt, 30*time.Second)
	assert.WithinDuration(t, before, service.UpdatedAt, 30*time.Second)
}

func fullSwarmServiceSpec(name string, replicas uint64) swarm.ServiceSpec {
	restartDelay := 100 * time.Millisecond
	maxAttempts := uint64(4)

	return swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: name,
			Labels: map[string]string{
				"service-label": "service-label-value",
			},
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: &swarm.ContainerSpec{
				Image:           "busybox:latest",
				Labels:          map[string]string{"container-label": "container-value"},
				Command:         []string{"/bin/top"},
				Args:            []string{"-u", "root"},
				Hostname:        "hostname",
				Env:             []string{"envvar=envvalue"},
				Dir:             "/work",
				User:            "root",
				StopSignal:      "SIGINT",
				StopGracePeriod: &restartDelay,
				Hosts:           []string{"8.8.8.8  google"},
				DNSConfig: &swarm.DNSConfig{
					Nameservers: []string{"8.8.8.8"},
					Search:      []string{"somedomain"},
				},
			},
			RestartPolicy: &swarm.RestartPolicy{
				Delay:       &restartDelay,
				Condition:   swarm.RestartPolicyConditionOnFailure,
				MaxAttempts: &maxAttempts,
			},
			Runtime: swarm.RuntimeContainer,
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &replicas,
			},
		},
		UpdateConfig: &swarm.UpdateConfig{
			Parallelism:     2,
			Delay:           200 * time.Second,
			FailureAction:   swarm.UpdateFailureActionContinue,
			Monitor:         2 * time.Second,
			MaxFailureRatio: 0.2,
			Order:           swarm.UpdateOrderStopFirst,
		},
		RollbackConfig: &swarm.UpdateConfig{
			Parallelism:     3,
			Delay:           300 * time.Second,
			FailureAction:   swarm.UpdateFailureActionPause,
			Monitor:         3 * time.Second,
			MaxFailureRatio: 0.3,
			Order:           swarm.UpdateOrderStartFirst,
		},
	}
}

const defaultSwarmPort = 2477

func newSwarm(t *testing.T) *daemon.Swarm {
	d := &daemon.Swarm{
		Daemon: daemon.New(t, "", dockerdBinary, daemon.Config{
			Experimental: testEnv.DaemonInfo.ExperimentalBuild,
		}),
		// TODO: better method of finding an unused port
		Port: defaultSwarmPort,
	}
	// TODO: move to a NewSwarm constructor
	d.ListenAddr = fmt.Sprintf("0.0.0.0:%d", d.Port)

	// avoid networking conflicts
	args := []string{"--iptables=false", "--swarm-default-advertise-addr=lo"}
	d.StartWithBusybox(t, args...)

	require.NoError(t, d.Init(swarm.InitRequest{}))
	return d
}

func serviceContainerCount(client client.ServiceAPIClient, id string, count uint64) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		filter := filters.NewArgs()
		filter.Add("service", id)
		tasks, err := client.TaskList(context.Background(), types.TaskListOptions{
			Filters: filter,
		})
		switch {
		case err != nil:
			return poll.Error(err)
		case len(tasks) == int(count):
			return poll.Success()
		default:
			return poll.Continue("task count at %d waiting for %d", len(tasks), count)
		}
	}
}
