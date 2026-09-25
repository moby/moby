package llbsolver

import (
	"context"

	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/sourcepolicy"
	spb "github.com/moby/buildkit/sourcepolicy/pb"
	"github.com/moby/buildkit/util/network"
	"github.com/pkg/errors"
)

const keyProxyNetwork = "llb.proxy-network"

func proxyNetworkForOp(op *pb.Op, proxyNetwork bool) (bool, error) {
	exec := op.GetExec()
	if exec == nil {
		return false, nil
	}
	switch exec.Network {
	case pb.NetMode_UNSET, pb.NetMode_HOST:
		return proxyNetwork, nil
	case pb.NetMode_NONE:
		return false, nil
	default:
		if proxyNetwork {
			return false, errors.Errorf("network mode %s is not allowed when proxy network is enabled", exec.Network)
		}
		return false, nil
	}
}

func (b *provenanceBridge) ProxyPolicy() (network.ProxyPolicy, error) {
	return b.llbBridge.ProxyPolicy()
}

func (b *provenanceBridge) ProxyNetwork() bool {
	return b.llbBridge.ProxyNetwork()
}

func (b *llbBridge) ProxyNetwork() bool {
	return b.proxyNetwork || loadProxyNetwork(b.builder)
}

func loadProxyNetwork(b solver.Builder) bool {
	if b == nil {
		return false
	}
	var proxyNetwork bool
	_ = b.EachValue(context.TODO(), keyProxyNetwork, func(v any) error {
		if v, ok := v.(bool); ok {
			proxyNetwork = proxyNetwork || v
		}
		return nil
	})
	return proxyNetwork
}

func (b *llbBridge) ProxyPolicy() (network.ProxyPolicy, error) {
	srcPol, err := loadSourcePolicy(b.builder)
	if err != nil {
		return nil, err
	}
	policySession, err := loadSourcePolicySession(b.builder)
	if err != nil {
		return nil, err
	}
	if (srcPol == nil || len(srcPol.Rules) == 0) && policySession == "" {
		return nil, nil
	}
	var policies []*spb.Policy
	if srcPol != nil {
		policies = append(policies, srcPol)
	}
	return b.policy(sourcepolicy.NewEngine(policies)), nil
}
