//go:build !linux

package libnetwork

import (
	"github.com/docker/go-events"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

func validateIngressSourceIPPlatform() error {
	return types.NotImplementedErrorf("preserving ingress source IPs is only supported on Linux")
}

func (*Endpoint) addIngressGatewayInfoToCluster() error { return nil }

func (*Endpoint) deleteIngressGatewayInfoFromCluster() error { return nil }

func (*Controller) handleIngressGatewayTableEvent(events.Event) {}

func (*Network) restoreIngressSourceIPRouting() {}
