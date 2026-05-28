package container

import (
	"errors"
	"fmt"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/daemon/internal/netiputil"
	"github.com/moby/swarmkit/v2/api"
)

func ipamConfig(ic *api.IPAMConfig) (network.IPAMConfig, error) {
	var (
		cfg  network.IPAMConfig
		errs []error
		err  error
	)
	cfg.Subnet, err = netiputil.MaybeParseCIDR(ic.Subnet)
	if err != nil {
		errs = append(errs, fmt.Errorf("invalid subnet: %w", err))
	}
	cfg.IPRange, err = netiputil.MaybeParseCIDR(ic.Range)
	if err != nil {
		errs = append(errs, fmt.Errorf("invalid ip range: %w", err))
	}
	gw, err := netiputil.MaybeParseAddr(ic.Gateway)
	cfg.Gateway = gw.Unmap()
	if err != nil {
		errs = append(errs, fmt.Errorf("invalid gateway: %w", err))
	}
	return cfg, errors.Join(errs...)
}
