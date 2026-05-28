package client

import (
	"context"

	"github.com/moby/moby/api/types/swarm"
)

// SwarmJoinOptions specifies options for joining a swarm.
type SwarmJoinOptions struct {
	ListenAddr    string
	AdvertiseAddr string
	DataPathAddr  string
	RemoteAddrs   []string
	JoinToken     string // accept by secret
	Availability  swarm.NodeAvailability
}

// SwarmJoinResult contains the result of joining a swarm.
type SwarmJoinResult struct {
	// No fields currently; placeholder for future use
}

// SwarmJoin joins the swarm.
func (cli *Client) SwarmJoin(ctx context.Context, options SwarmJoinOptions) (SwarmJoinResult, error) {
	req := swarm.JoinRequest{
		ListenAddr:    options.ListenAddr,
		AdvertiseAddr: options.AdvertiseAddr,
		DataPathAddr:  options.DataPathAddr,
		RemoteAddrs:   options.RemoteAddrs,
		JoinToken:     options.JoinToken,
		Availability:  options.Availability,
	}

	resp, err := cli.post(ctx, "/swarm/join", nil, req, nil)
	defer ensureReaderClosed(resp)
	return SwarmJoinResult{}, err
}
