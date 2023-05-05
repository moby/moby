//go:build !windows

package network

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
)

// CreateMasterDummy creates a dummy network interface
func CreateMasterDummy(t *testing.T, master string) {
	// ip link add <dummy_name> type dummy
	icmd.RunCommand("ip", "link", "add", master, "type", "dummy").Assert(t, icmd.Success)
	icmd.RunCommand("ip", "link", "set", master, "up").Assert(t, icmd.Success)
}

// CreateVlanInterface creates a vlan network interface
func CreateVlanInterface(t *testing.T, master, slave, id string) {
	// ip link add link <master> name <master>.<VID> type vlan id <VID>
	icmd.RunCommand("ip", "link", "add", "link", master, "name", slave, "type", "vlan", "id", id).Assert(t, icmd.Success)
	// ip link set <sub_interface_name> up
	icmd.RunCommand("ip", "link", "set", slave, "up").Assert(t, icmd.Success)
}

// DeleteInterface deletes a network interface
func DeleteInterface(t *testing.T, ifName string) {
	icmd.RunCommand("ip", "link", "delete", ifName).Assert(t, icmd.Success)
	icmd.RunCommand("iptables", "-t", "nat", "--flush").Assert(t, icmd.Success)
	icmd.RunCommand("iptables", "--flush").Assert(t, icmd.Success)
}

// LinkExists verifies that a link exists
func LinkExists(t *testing.T, master string) {
	// verify the specified link exists, ip link show <link_name>
	icmd.RunCommand("ip", "link", "show", master).Assert(t, icmd.Success)
}

// IsNetworkAvailable provides a comparison to check if a docker network is available
func IsNetworkAvailable(c client.NetworkAPIClient, name string) cmp.Comparison {
	return func() cmp.Result {
		networks, err := c.NetworkList(context.Background(), types.NetworkListOptions{})
		if err != nil {
			return cmp.ResultFromError(err)
		}
		for _, network := range networks {
			if network.Name == name {
				return cmp.ResultSuccess
			}
		}
		return cmp.ResultFailure(fmt.Sprintf("could not find network %s", name))
	}
}

// IsNetworkNotAvailable provides a comparison to check if a docker network is not available
func IsNetworkNotAvailable(c client.NetworkAPIClient, name string) cmp.Comparison {
	return func() cmp.Result {
		networks, err := c.NetworkList(context.Background(), types.NetworkListOptions{})
		if err != nil {
			return cmp.ResultFromError(err)
		}
		for _, network := range networks {
			if network.Name == name {
				return cmp.ResultFailure(fmt.Sprintf("network %s is still present", name))
			}
		}
		return cmp.ResultSuccess
	}
}
