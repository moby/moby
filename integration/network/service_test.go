package network // import "github.com/docker/docker/integration/network"

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/gotestyourself/gotestyourself/assert"
	"github.com/gotestyourself/gotestyourself/poll"
	"golang.org/x/net/context"
)

func TestServiceWithPredefinedNetwork(t *testing.T) {
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	hostName := "host"
	var instances uint64 = 1
	serviceName := "TestService"
	serviceSpec := swarmServiceSpec(serviceName, instances)
	serviceSpec.TaskTemplate.Networks = append(serviceSpec.TaskTemplate.Networks, swarmtypes.NetworkAttachmentConfig{Target: hostName})

	serviceResp, err := client.ServiceCreate(context.Background(), serviceSpec, types.ServiceCreateOptions{
		QueryRegistry: false,
	})
	assert.NilError(t, err)

	serviceID := serviceResp.ID
	poll.WaitOn(t, serviceRunningCount(client, serviceID, instances), swarm.ServicePoll)

	_, _, err = client.ServiceInspectWithRaw(context.Background(), serviceID, types.ServiceInspectOptions{})
	assert.NilError(t, err)

	err = client.ServiceRemove(context.Background(), serviceID)
	assert.NilError(t, err)
}

const ingressNet = "ingress"

func TestServiceWithIngressNetwork(t *testing.T) {
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	poll.WaitOn(t, swarmIngressReady(client), swarm.NetworkPoll)

	var instances uint64 = 1
	serviceName := "TestIngressService"
	serviceSpec := swarmServiceSpec(serviceName, instances)
	serviceSpec.TaskTemplate.Networks = append(serviceSpec.TaskTemplate.Networks, swarmtypes.NetworkAttachmentConfig{Target: ingressNet})
	serviceSpec.EndpointSpec = &swarmtypes.EndpointSpec{
		Ports: []swarmtypes.PortConfig{
			{
				Protocol:    swarmtypes.PortConfigProtocolTCP,
				TargetPort:  80,
				PublishMode: swarmtypes.PortConfigPublishModeIngress,
			},
		},
	}

	serviceResp, err := client.ServiceCreate(context.Background(), serviceSpec, types.ServiceCreateOptions{
		QueryRegistry: false,
	})
	assert.NilError(t, err)

	serviceID := serviceResp.ID
	poll.WaitOn(t, serviceRunningCount(client, serviceID, instances), swarm.ServicePoll)

	_, _, err = client.ServiceInspectWithRaw(context.Background(), serviceID, types.ServiceInspectOptions{})
	assert.NilError(t, err)

	err = client.ServiceRemove(context.Background(), serviceID)
	assert.NilError(t, err)

	poll.WaitOn(t, serviceIsRemoved(client, serviceID), swarm.ServicePoll)
	poll.WaitOn(t, noServices(client), swarm.ServicePoll)

	// Ensure that "ingress" is not removed or corrupted
	time.Sleep(10 * time.Second)
	netInfo, err := client.NetworkInspect(context.Background(), ingressNet, types.NetworkInspectOptions{
		Verbose: true,
		Scope:   "swarm",
	})
	assert.NilError(t, err, "Ingress network was removed after removing service!")
	assert.Assert(t, len(netInfo.Containers) != 0, "No load balancing endpoints in ingress network")
	assert.Assert(t, len(netInfo.Peers) != 0, "No peers (including self) in ingress network")
	_, ok := netInfo.Containers["ingress-sbox"]
	assert.Assert(t, ok, "ingress-sbox not present in ingress network")
}

func serviceRunningCount(client client.ServiceAPIClient, serviceID string, instances uint64) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
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

func swarmIngressReady(client client.NetworkAPIClient) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		netInfo, err := client.NetworkInspect(context.Background(), ingressNet, types.NetworkInspectOptions{
			Verbose: true,
			Scope:   "swarm",
		})
		if err != nil {
			return poll.Error(err)
		}
		np := len(netInfo.Peers)
		nc := len(netInfo.Containers)
		if np == 0 || nc == 0 {
			return poll.Continue("ingress not ready: %d peers and %d containers", nc, np)
		}
		_, ok := netInfo.Containers["ingress-sbox"]
		if !ok {
			return poll.Continue("ingress not ready: does not contain the ingress-sbox")
		}
		return poll.Success()
	}
}

func noServices(client client.ServiceAPIClient) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		services, err := client.ServiceList(context.Background(), types.ServiceListOptions{})
		switch {
		case err != nil:
			return poll.Error(err)
		case len(services) == 0:
			return poll.Success()
		default:
			return poll.Continue("Service count at %d waiting for 0", len(services))
		}
	}
}
