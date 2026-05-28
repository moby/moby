//go:generate protoc --gogofaster_out=. extra.proto

package netextra

import (
	"net/netip"

	"github.com/gogo/protobuf/types"
	"github.com/moby/moby/api/types/network"
)

func OptionsFrom(typeurl string, value []byte) (GetNetworkExtraOptions, error) {
	if typeurl == "" {
		return GetNetworkExtraOptions{}, nil
	}
	appdata := &types.Any{TypeUrl: typeurl, Value: value}

	var xo GetNetworkExtraOptions
	if !types.Is(appdata, &xo) {
		// Forward-compatibility: ignore unknown message types
		return GetNetworkExtraOptions{}, nil
	}
	if err := types.UnmarshalAny(appdata, &xo); err != nil {
		return GetNetworkExtraOptions{}, err
	}
	return xo, nil
}

func StatusFrom(extra *types.Any) (*network.Status, error) {
	if extra == nil {
		return nil, nil
	}

	var x Extra
	if !types.Is(extra, &x) {
		// Forward-compatibility: ignore unknown message types
		return nil, nil
	}
	if err := types.UnmarshalAny(extra, &x); err != nil {
		return nil, err
	}

	status := network.Status{
		IPAM: network.IPAMStatus{
			Subnets: make(map[netip.Prefix]network.SubnetStatus, len(x.IPAMStatus)),
		},
	}

	for _, s := range x.IPAMStatus {
		var pfx netip.Prefix
		err := pfx.UnmarshalBinary(s.Subnet)
		if err != nil {
			return nil, err
		}
		status.IPAM.Subnets[pfx] = network.SubnetStatus{
			IPsInUse:            s.IPsInUse,
			DynamicIPsAvailable: s.DynamicIPsAvailable,
		}
	}

	return &status, nil
}

func MarshalStatus(status *network.Status) (*types.Any, error) {
	if status == nil {
		return nil, nil
	}

	var ipam []*IPAMStatus
	for subnet, s := range status.IPAM.Subnets {
		bpfx, err := subnet.MarshalBinary()
		if err != nil {
			return nil, err
		}
		ipam = append(ipam, &IPAMStatus{
			Subnet:              bpfx,
			IPsInUse:            s.IPsInUse,
			DynamicIPsAvailable: s.DynamicIPsAvailable,
		})
	}
	return types.MarshalAny(&Extra{
		IPAMStatus: ipam,
	})
}
