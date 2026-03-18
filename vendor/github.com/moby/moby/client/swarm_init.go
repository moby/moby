package client

import (
	"context"
	"encoding/json"
	"net/netip"

	"github.com/moby/moby/api/types/swarm"
)

// SwarmInitOptions contains options for initializing a new swarm.
type SwarmInitOptions struct {
	ListenAddr       string
	AdvertiseAddr    string
	DataPathAddr     string
	DataPathPort     uint32
	ForceNewCluster  bool
	Spec             swarm.Spec
	AutoLockManagers bool
	Availability     swarm.NodeAvailability
	DefaultAddrPool  []netip.Prefix
	SubnetSize       uint32
}

// SwarmInitResult contains the result of a SwarmInit operation.
type SwarmInitResult struct {
	NodeID string
}

// SwarmInit initializes the swarm.
func (cli *Client) SwarmInit(ctx context.Context, options SwarmInitOptions) (SwarmInitResult, error) {
	req := swarm.InitRequest{
		ListenAddr:       options.ListenAddr,
		AdvertiseAddr:    options.AdvertiseAddr,
		DataPathAddr:     options.DataPathAddr,
		DataPathPort:     options.DataPathPort,
		ForceNewCluster:  options.ForceNewCluster,
		Spec:             options.Spec,
		AutoLockManagers: options.AutoLockManagers,
		Availability:     options.Availability,
		DefaultAddrPool:  options.DefaultAddrPool,
		SubnetSize:       options.SubnetSize,
	}

	resp, err := cli.post(ctx, "/swarm/init", nil, req, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return SwarmInitResult{}, err
	}

	var nodeID string
	err = json.NewDecoder(resp.Body).Decode(&nodeID)
	return SwarmInitResult{NodeID: nodeID}, err
}
