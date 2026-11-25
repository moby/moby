//go:generate protoc -I=../../../.. --go_out=../../../.. --go_opt=paths=source_relative --go-vtproto_out=../../../.. --go-vtproto_opt=paths=source_relative,features=marshal+unmarshal+size daemon/cluster/convert/netextra/extra.proto

package netextra

import (
	"net/netip"

	"github.com/gogo/protobuf/types"
	"github.com/moby/moby/api/types/network"
	"google.golang.org/protobuf/proto"
)

type vtMessage interface {
	proto.Message
	MarshalVT() ([]byte, error)
}

// marshalAny encodes m with the type URL that [types.MarshalAny] produced,
// which these messages were encoded with before they moved off gogo/protobuf.
func marshalAny(m vtMessage) (*types.Any, error) {
	value, err := m.MarshalVT()
	if err != nil {
		return nil, err
	}

	return &types.Any{
		TypeUrl: "type.googleapis.com/" + string(m.ProtoReflect().Descriptor().FullName()),
		Value:   value,
	}, nil
}

// isType reports whether typeurl names m's message type, matching only the
// part after the last '/' as [types.Is] does.
func isType(typeurl string, m proto.Message) bool {
	name := string(m.ProtoReflect().Descriptor().FullName())
	prefix := len(typeurl) - len(name)

	return prefix >= 1 && typeurl[prefix-1] == '/' && typeurl[prefix:] == name
}

// MarshalOptions encodes opts as appdata for the Swarm network API.
func MarshalOptions(opts *GetNetworkExtraOptions) (*types.Any, error) {
	return marshalAny(opts)
}

func OptionsFrom(typeurl string, value []byte) (*GetNetworkExtraOptions, error) {
	var xo GetNetworkExtraOptions
	if !isType(typeurl, &xo) {
		// Forward-compatibility: ignore unknown message types
		return &xo, nil
	}

	if err := xo.UnmarshalVT(value); err != nil {
		return nil, err
	}

	return &xo, nil
}

func StatusFrom(extra *types.Any) (*network.Status, error) {
	if extra == nil {
		return nil, nil
	}

	var x Extra
	if !isType(extra.TypeUrl, &x) {
		// Forward-compatibility: ignore unknown message types
		return nil, nil
	}

	if err := x.UnmarshalVT(extra.Value); err != nil {
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

	return marshalAny(&Extra{IPAMStatus: ipam})
}
