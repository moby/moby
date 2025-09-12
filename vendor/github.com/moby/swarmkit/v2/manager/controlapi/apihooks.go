package controlapi

import (
	"context"

	"github.com/moby/swarmkit/v2/api"
)

// NetworkViewResponseMutator provides callbacks which may modify the response
// objects for GetNetwork and ListNetworks Control API requests before they are
// sent to the client.
type NetworkViewResponseMutator interface {
	OnGetNetwork(context.Context, *api.Network, string, []byte) error
	OnListNetworks(context.Context, []*api.Network, string, []byte) error
}

type NoopViewResponseMutator struct{}

func (NoopViewResponseMutator) OnGetNetwork(ctx context.Context, n *api.Network, appdataTypeURL string, appdata []byte) error {
	return nil
}

func (NoopViewResponseMutator) OnListNetworks(ctx context.Context, networks []*api.Network, appdataTypeURL string, appdata []byte) error {
	return nil
}

func (s *Server) networkhooks() NetworkViewResponseMutator {
	if s.NetworkHooks != nil {
		return s.NetworkHooks
	}
	return NoopViewResponseMutator{}
}
