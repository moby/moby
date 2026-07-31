//go:generate protoc --gogofaster_out=. extra.proto

package netextra

import (
	"net/netip"
	"strings"

	"github.com/moby/moby/api/types/network"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	optionsTypeURL = "type.googleapis.com/docker.engine.netextra.GetNetworkExtraOptions"
	extraTypeURL   = "type.googleapis.com/docker.engine.netextra.Extra"
)

func OptionsFrom(typeurl string, value []byte) (GetNetworkExtraOptions, error) {
	if typeurl == "" {
		return GetNetworkExtraOptions{}, nil
	}
	var xo GetNetworkExtraOptions
	if !strings.HasSuffix(typeurl, "/docker.engine.netextra.GetNetworkExtraOptions") {
		// Forward-compatibility: ignore unknown message types
		return GetNetworkExtraOptions{}, nil
	}
	if err := xo.Unmarshal(value); err != nil {
		return GetNetworkExtraOptions{}, err
	}
	return xo, nil
}

func MarshalOptions(options *GetNetworkExtraOptions) (*anypb.Any, error) {
	value, err := options.Marshal()
	if err != nil {
		return nil, err
	}
	return &anypb.Any{TypeUrl: optionsTypeURL, Value: value}, nil
}

func StatusFrom(extra *anypb.Any) (*network.Status, error) {
	if extra == nil {
		return nil, nil
	}

	var x Extra
	if !strings.HasSuffix(extra.TypeUrl, "/docker.engine.netextra.Extra") {
		// Forward-compatibility: ignore unknown message types
		return nil, nil
	}
	if err := x.Unmarshal(extra.Value); err != nil {
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

func MarshalStatus(status *network.Status) (*anypb.Any, error) {
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
	value, err := (&Extra{
		IPAMStatus: ipam,
	}).Marshal()
	if err != nil {
		return nil, err
	}
	return &anypb.Any{TypeUrl: extraTypeURL, Value: value}, nil
}
