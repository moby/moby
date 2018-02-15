package network // import "github.com/docker/docker/integration/network"

import (
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestServiceWithPredefinedNetwork(t *testing.T) {
	defer setupTest(t)()
	d := newSwarm(t)
	defer d.Stop(t)
	client, err := client.NewClientWithOpts(client.WithHost((d.Sock())))
	require.NoError(t, err)

	hostName := "host"
	var instances uint64 = 1
	serviceName := "TestService"
	serviceSpec := swarmServiceSpec(serviceName, instances)
	serviceSpec.TaskTemplate.Networks = append(serviceSpec.TaskTemplate.Networks, swarm.NetworkAttachmentConfig{Target: hostName})

	serviceResp, err := client.ServiceCreate(context.Background(), serviceSpec, types.ServiceCreateOptions{
		QueryRegistry: false,
	})
	require.NoError(t, err)

	pollSettings := func(config *poll.Settings) {
		if runtime.GOARCH == "arm64" || runtime.GOARCH == "arm" {
			config.Timeout = 50 * time.Second
			config.Delay = 100 * time.Millisecond
		}
	}

	serviceID := serviceResp.ID
	poll.WaitOn(t, serviceRunningCount(client, serviceID, instances), pollSettings)

	_, _, err = client.ServiceInspectWithRaw(context.Background(), serviceID, types.ServiceInspectOptions{})
	require.NoError(t, err)

	err = client.ServiceRemove(context.Background(), serviceID)
	require.NoError(t, err)

	poll.WaitOn(t, serviceIsRemoved(client, serviceID), pollSettings)
	poll.WaitOn(t, noTasks(client), pollSettings)

}

func serviceRunningCount(client client.ServiceAPIClient, serviceID string, instances uint64) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		filter := filters.NewArgs()
		filter.Add("service", serviceID)
		services, err := client.ServiceList(context.Background(), types.ServiceListOptions{})
		if err != nil {
			return poll.Error(err)
		}

		if len(services) != int(instances) {
			return poll.Continue("Service count at %d waiting for %d", len(services), instances)
		}
		return poll.Success()
	}
}
