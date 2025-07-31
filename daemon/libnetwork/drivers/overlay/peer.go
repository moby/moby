package overlay

import (
	"fmt"
	"net/netip"

	"github.com/gogo/protobuf/proto"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/hashable"
)

// OverlayPeerTable is the NetworkDB table for overlay network peer discovery.
const OverlayPeerTable = "overlay_peer_table"

type Peer struct {
	EndpointIP       netip.Prefix
	EndpointMAC      hashable.MACAddr
	TunnelEndpointIP netip.Addr
}

func UnmarshalPeerRecord(data []byte) (*Peer, error) {
	var pr PeerRecord
	if err := proto.Unmarshal(data, &pr); err != nil {
		return nil, fmt.Errorf("failed to unmarshal peer record: %w", err)
	}
	var (
		p   Peer
		err error
	)
	p.EndpointIP, err = netip.ParsePrefix(pr.EndpointIP)
	if err != nil {
		return nil, fmt.Errorf("invalid peer IP %q received: %w", pr.EndpointIP, err)
	}
	p.EndpointMAC, err = hashable.ParseMAC(pr.EndpointMAC)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC %q received: %w", pr.EndpointMAC, err)
	}
	p.TunnelEndpointIP, err = netip.ParseAddr(pr.TunnelEndpointIP)
	if err != nil {
		return nil, fmt.Errorf("invalid VTEP %q received: %w", pr.TunnelEndpointIP, err)
	}
	return &p, nil
}
