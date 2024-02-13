package netproviders

import (
	"os"

	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	"github.com/moby/buildkit/util/network/cniprovider"
	"github.com/pkg/errors"
)

type Opt struct {
	CNI  cniprovider.Opt
	Mode string
}

// Providers returns the network provider set.
// When opt.Mode is "auto" or "", resolvedMode is set to either "cni" or "host".
func Providers(opt Opt) (providers map[pb.NetMode]network.Provider, resolvedMode string, err error) {
	var defaultProvider network.Provider
	switch opt.Mode {
	case "cni":
		cniProvider, err := cniprovider.New(opt.CNI)
		if err != nil {
			return nil, resolvedMode, err
		}
		defaultProvider = cniProvider
		resolvedMode = opt.Mode
	case "host":
		hostProvider, ok := getHostProvider()
		if !ok {
			return nil, resolvedMode, errors.New("no host network support on this platform")
		}
		defaultProvider = hostProvider
		resolvedMode = opt.Mode
	case "auto", "":
		if _, err := os.Stat(opt.CNI.ConfigPath); err == nil {
			cniProvider, err := cniprovider.New(opt.CNI)
			if err != nil {
				return nil, resolvedMode, err
			}
			defaultProvider = cniProvider
			resolvedMode = "cni"
		} else {
			defaultProvider, resolvedMode = getFallback()
		}
	default:
		return nil, resolvedMode, errors.Errorf("invalid network mode: %q", opt.Mode)
	}

	providers = map[pb.NetMode]network.Provider{
		pb.NetMode_UNSET: defaultProvider,
		pb.NetMode_NONE:  network.NewNoneProvider(),
	}

	if hostProvider, ok := getHostProvider(); ok {
		providers[pb.NetMode_HOST] = hostProvider
	}

	return providers, resolvedMode, nil
}
