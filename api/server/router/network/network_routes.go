package network

import (
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/runconfig"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	"github.com/docker/engine-api/types/network"
	"github.com/docker/libnetwork"
)

func (n *networkRouter) getNetworksList(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	filter := r.Form.Get("filters")
	netFilters, err := filters.FromParam(filter)
	if err != nil {
		return err
	}

	if netFilters.Len() != 0 {
		if err := netFilters.Validate(acceptedFilters); err != nil {
			return err
		}
	}

	list := []*types.NetworkResource{}

	nwList := n.backend.GetAllNetworks()
	displayable, err := filterNetworks(nwList, netFilters)
	if err != nil {
		return err
	}

	for _, nw := range displayable {
		list = append(list, buildNetworkResource(nw))
	}

	return httputils.WriteJSON(w, http.StatusOK, list)
}

func (n *networkRouter) getNetwork(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	nw, err := n.backend.FindNetwork(vars["id"])
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, buildNetworkResource(nw))
}

func (n *networkRouter) postNetworkCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var create types.NetworkCreate
	var warning string

	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
		return err
	}

	if runconfig.IsPreDefinedNetwork(create.Name) {
		return httputils.WriteJSON(w, http.StatusForbidden,
			fmt.Sprintf("%s is a pre-defined network and cannot be created", create.Name))
	}

	nw, err := n.backend.GetNetworkByName(create.Name)
	if _, ok := err.(libnetwork.ErrNoSuchNetwork); err != nil && !ok {
		return err
	}
	if nw != nil {
		if create.CheckDuplicate {
			return libnetwork.NetworkNameError(create.Name)
		}
		warning = fmt.Sprintf("Network with name %s (id : %s) already exists", nw.Name(), nw.ID())
	}

	nw, err = n.backend.CreateNetwork(create.Name, create.Driver, create.IPAM, create.Options, create.Internal, create.EnableIPv6)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusCreated, &types.NetworkCreateResponse{
		ID:      nw.ID(),
		Warning: warning,
	})
}

func (n *networkRouter) postNetworkConnect(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var connect types.NetworkConnect
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	if err := json.NewDecoder(r.Body).Decode(&connect); err != nil {
		return err
	}

	nw, err := n.backend.FindNetwork(vars["id"])
	if err != nil {
		return err
	}

	return n.backend.ConnectContainerToNetwork(connect.Container, nw.Name(), connect.EndpointConfig)
}

func (n *networkRouter) postNetworkDisconnect(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var disconnect types.NetworkDisconnect
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	if err := json.NewDecoder(r.Body).Decode(&disconnect); err != nil {
		return err
	}

	nw, err := n.backend.FindNetwork(vars["id"])
	if err != nil {
		return err
	}

	return n.backend.DisconnectContainerFromNetwork(disconnect.Container, nw, disconnect.Force)
}

func (n *networkRouter) deleteNetwork(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return n.backend.DeleteNetwork(vars["id"])
}

func buildNetworkResource(nw libnetwork.Network) *types.NetworkResource {
	r := &types.NetworkResource{}
	if nw == nil {
		return r
	}

	r.Name = nw.Name()
	r.ID = nw.ID()
	r.Scope = nw.Info().Scope()
	r.Driver = nw.Type()
	r.EnableIPv6 = nw.Info().IPv6Enabled()
	r.Internal = nw.Info().Internal()
	r.Options = nw.Info().DriverOptions()
	r.Containers = make(map[string]types.EndpointResource)
	buildIpamResources(r, nw)
	r.Internal = nw.Info().Internal()

	epl := nw.Endpoints()
	for _, e := range epl {
		ei := e.Info()
		if ei == nil {
			continue
		}
		sb := ei.Sandbox()
		key := "ep-" + e.ID()
		if sb != nil {
			key = sb.ContainerID()
		}

		r.Containers[key] = buildEndpointResource(e)
	}
	return r
}

func buildIpamResources(r *types.NetworkResource, nw libnetwork.Network) {
	id, opts, ipv4conf, ipv6conf := nw.Info().IpamConfig()

	ipv4Info, ipv6Info := nw.Info().IpamInfo()

	r.IPAM.Driver = id

	r.IPAM.Options = opts

	r.IPAM.Config = []network.IPAMConfig{}
	for _, ip4 := range ipv4conf {
		if ip4.PreferredPool == "" {
			continue
		}
		iData := network.IPAMConfig{}
		iData.Subnet = ip4.PreferredPool
		iData.IPRange = ip4.SubPool
		iData.Gateway = ip4.Gateway
		iData.AuxAddress = ip4.AuxAddresses
		r.IPAM.Config = append(r.IPAM.Config, iData)
	}

	if len(r.IPAM.Config) == 0 {
		for _, ip4Info := range ipv4Info {
			iData := network.IPAMConfig{}
			iData.Subnet = ip4Info.IPAMData.Pool.String()
			iData.Gateway = ip4Info.IPAMData.Gateway.String()
			r.IPAM.Config = append(r.IPAM.Config, iData)
		}
	}

	hasIpv6Conf := false
	for _, ip6 := range ipv6conf {
		if ip6.PreferredPool == "" {
			continue
		}
		hasIpv6Conf = true
		iData := network.IPAMConfig{}
		iData.Subnet = ip6.PreferredPool
		iData.IPRange = ip6.SubPool
		iData.Gateway = ip6.Gateway
		iData.AuxAddress = ip6.AuxAddresses
		r.IPAM.Config = append(r.IPAM.Config, iData)
	}

	if !hasIpv6Conf {
		for _, ip6Info := range ipv6Info {
			iData := network.IPAMConfig{}
			iData.Subnet = ip6Info.IPAMData.Pool.String()
			iData.Gateway = ip6Info.IPAMData.Gateway.String()
			r.IPAM.Config = append(r.IPAM.Config, iData)
		}
	}
}

func buildEndpointResource(e libnetwork.Endpoint) types.EndpointResource {
	er := types.EndpointResource{}
	if e == nil {
		return er
	}

	er.EndpointID = e.ID()
	er.Name = e.Name()
	ei := e.Info()
	if ei == nil {
		return er
	}

	if iface := ei.Iface(); iface != nil {
		if mac := iface.MacAddress(); mac != nil {
			er.MacAddress = mac.String()
		}
		if ip := iface.Address(); ip != nil && len(ip.IP) > 0 {
			er.IPv4Address = ip.String()
		}

		if ipv6 := iface.AddressIPv6(); ipv6 != nil && len(ipv6.IP) > 0 {
			er.IPv6Address = ipv6.String()
		}
	}
	return er
}
