package network

import (
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemon/network"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/runconfig"
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

	list := []*types.NetworkResource{}
	var nameFilter, idFilter bool
	var names, ids []string
	if names, nameFilter = netFilters["name"]; nameFilter {
		for _, name := range names {
			if nw, err := n.backend.GetNetwork(name, daemon.NetworkByName); err == nil {
				list = append(list, buildNetworkResource(nw))
			} else {
				logrus.Errorf("failed to get network for filter=%s : %v", name, err)
			}
		}
	}

	if ids, idFilter = netFilters["id"]; idFilter {
		for _, id := range ids {
			for _, nw := range n.backend.GetNetworksByID(id) {
				list = append(list, buildNetworkResource(nw))
			}
		}
	}

	if !nameFilter && !idFilter {
		nwList := n.backend.GetNetworksByID("")
		for _, nw := range nwList {
			list = append(list, buildNetworkResource(nw))
		}
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

	nw, err := n.backend.GetNetwork(create.Name, daemon.NetworkByName)
	if _, ok := err.(libnetwork.ErrNoSuchNetwork); err != nil && !ok {
		return err
	}
	if nw != nil {
		if create.CheckDuplicate {
			return libnetwork.NetworkNameError(create.Name)
		}
		warning = fmt.Sprintf("Network with name %s (id : %s) already exists", nw.Name(), nw.ID())
	}

	nw, err = n.backend.CreateNetwork(create.Name, create.Driver, create.IPAM, create.Options)
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

	return n.backend.ConnectContainerToNetwork(connect.Container, nw.Name())
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

	return n.backend.DisconnectContainerFromNetwork(disconnect.Container, nw)
}

func (n *networkRouter) deleteNetwork(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	nw, err := n.backend.FindNetwork(vars["id"])
	if err != nil {
		return err
	}

	if runconfig.IsPreDefinedNetwork(nw.Name()) {
		return httputils.WriteJSON(w, http.StatusForbidden,
			fmt.Sprintf("%s is a pre-defined network and cannot be removed", nw.Name()))
	}

	return nw.Delete()
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
	r.Options = nw.Info().DriverOptions()
	r.Containers = make(map[string]types.EndpointResource)
	buildIpamResources(r, nw)

	epl := nw.Endpoints()
	for _, e := range epl {
		ei := e.Info()
		if ei == nil {
			continue
		}
		sb := ei.Sandbox()
		if sb == nil {
			continue
		}

		r.Containers[sb.ContainerID()] = buildEndpointResource(e)
	}
	return r
}

func buildIpamResources(r *types.NetworkResource, nw libnetwork.Network) {
	id, ipv4conf, ipv6conf := nw.Info().IpamConfig()

	r.IPAM.Driver = id

	r.IPAM.Config = []network.IPAMConfig{}
	for _, ip4 := range ipv4conf {
		iData := network.IPAMConfig{}
		iData.Subnet = ip4.PreferredPool
		iData.IPRange = ip4.SubPool
		iData.Gateway = ip4.Gateway
		iData.AuxAddress = ip4.AuxAddresses
		r.IPAM.Config = append(r.IPAM.Config, iData)
	}

	for _, ip6 := range ipv6conf {
		iData := network.IPAMConfig{}
		iData.Subnet = ip6.PreferredPool
		iData.IPRange = ip6.SubPool
		iData.Gateway = ip6.Gateway
		iData.AuxAddress = ip6.AuxAddresses
		r.IPAM.Config = append(r.IPAM.Config, iData)
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
