package daemon

import (
	"testing"

	"github.com/Microsoft/hcsshim"
	containertypes "github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestHNSStatsToNetworkStats(t *testing.T) {
	in := &hcsshim.HNSEndpointStats{
		EndpointID:             "endpoint-1",
		InstanceID:             "instance-1",
		BytesReceived:          100,
		PacketsReceived:        10,
		DroppedPacketsIncoming: 1,
		BytesSent:              200,
		PacketsSent:            20,
		DroppedPacketsOutgoing: 2,
	}

	got := hnsStatsToNetworkStats(in)

	assert.Check(t, is.DeepEqual(got, containertypes.NetworkStats{
		RxBytes:    100,
		RxPackets:  10,
		RxDropped:  1,
		TxBytes:    200,
		TxPackets:  20,
		TxDropped:  2,
		EndpointID: "endpoint-1",
		InstanceID: "instance-1",
		// RxErrors/TxErrors are not reported by HNS and remain zero.
	}))
}
