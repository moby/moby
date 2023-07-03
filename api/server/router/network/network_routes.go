package network // import "github.com/docker/docker/api/server/router/network"

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/pkg/errors"
)

func (n *networkRouter) getNetworksList(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	filter, err := filters.FromJSON(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	if err := network.ValidateFilters(filter); err != nil {
		return err
	}

	var list []types.NetworkResource
	nr, err := n.cluster.GetNetworks(filter)
	if err == nil {
		list = nr
	}

	// Combine the network list returned by Docker daemon if it is not already
	// returned by the cluster manager
	localNetworks, err := n.backend.GetNetworks(filter, types.NetworkListConfig{Detailed: versions.LessThan(httputils.VersionFromContext(ctx), "1.28")})
	if err != nil {
		return err
	}

	var idx map[string]bool
	if len(list) > 0 {
		idx = make(map[string]bool, len(list))
		for _, n := range list {
			idx[n.ID] = true
		}
	}
	for _, n := range localNetworks {
		if idx[n.ID] {
			continue
		}
		list = append(list, n)
	}

	if list == nil {
		list = []types.NetworkResource{}
	}

	return httputils.WriteJSON(w, http.StatusOK, list)
}

type invalidRequestError struct {
	cause error
}

func (e invalidRequestError) Error() string {
	return e.cause.Error()
}

func (e invalidRequestError) InvalidParameter() {}

type ambigousResultsError string

func (e ambigousResultsError) Error() string {
	return "network " + string(e) + " is ambiguous"
}

func (ambigousResultsError) InvalidParameter() {}

func nameConflict(name string) error {
	return errdefs.Conflict(libnetwork.NetworkNameError(name))
}

func (n *networkRouter) getNetwork(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	term := vars["id"]
	var (
		verbose bool
		err     error
	)
	if v := r.URL.Query().Get("verbose"); v != "" {
		if verbose, err = strconv.ParseBool(v); err != nil {
			return errors.Wrapf(invalidRequestError{err}, "invalid value for verbose: %s", v)
		}
	}
	scope := r.URL.Query().Get("scope")

	// In case multiple networks have duplicate names, return error.
	// TODO (yongtang): should we wrap with version here for backward compatibility?

	// First find based on full ID, return immediately once one is found.
	// If a network appears both in swarm and local, assume it is in local first

	// For full name and partial ID, save the result first, and process later
	// in case multiple records was found based on the same term
	listByFullName := map[string]types.NetworkResource{}
	listByPartialID := map[string]types.NetworkResource{}

	// TODO(@cpuguy83): All this logic for figuring out which network to return does not belong here
	// Instead there should be a backend function to just get one network.
	filter := filters.NewArgs(filters.Arg("idOrName", term))
	if scope != "" {
		filter.Add("scope", scope)
	}
	nw, _ := n.backend.GetNetworks(filter, types.NetworkListConfig{Detailed: true, Verbose: verbose})
	for _, network := range nw {
		if network.ID == term {
			return httputils.WriteJSON(w, http.StatusOK, network)
		}
		if network.Name == term {
			// No need to check the ID collision here as we are still in
			// local scope and the network ID is unique in this scope.
			listByFullName[network.ID] = network
		}
		if strings.HasPrefix(network.ID, term) {
			// No need to check the ID collision here as we are still in
			// local scope and the network ID is unique in this scope.
			listByPartialID[network.ID] = network
		}
	}

	nwk, err := n.cluster.GetNetwork(term)
	if err == nil {
		// If the get network is passed with a specific network ID / partial network ID
		// or if the get network was passed with a network name and scope as swarm
		// return the network. Skipped using isMatchingScope because it is true if the scope
		// is not set which would be case if the client API v1.30
		if strings.HasPrefix(nwk.ID, term) || (datastore.SwarmScope == scope) {
			// If we have a previous match "backend", return it, we need verbose when enabled
			// ex: overlay/partial_ID or name/swarm_scope
			if nwv, ok := listByPartialID[nwk.ID]; ok {
				nwk = nwv
			} else if nwv, ok := listByFullName[nwk.ID]; ok {
				nwk = nwv
			}
			return httputils.WriteJSON(w, http.StatusOK, nwk)
		}
	}

	nr, _ := n.cluster.GetNetworks(filter)
	for _, network := range nr {
		if network.ID == term {
			return httputils.WriteJSON(w, http.StatusOK, network)
		}
		if network.Name == term {
			// Check the ID collision as we are in swarm scope here, and
			// the map (of the listByFullName) may have already had a
			// network with the same ID (from local scope previously)
			if _, ok := listByFullName[network.ID]; !ok {
				listByFullName[network.ID] = network
			}
		}
		if strings.HasPrefix(network.ID, term) {
			// Check the ID collision as we are in swarm scope here, and
			// the map (of the listByPartialID) may have already had a
			// network with the same ID (from local scope previously)
			if _, ok := listByPartialID[network.ID]; !ok {
				listByPartialID[network.ID] = network
			}
		}
	}

	// Find based on full name, returns true only if no duplicates
	if len(listByFullName) == 1 {
		for _, v := range listByFullName {
			return httputils.WriteJSON(w, http.StatusOK, v)
		}
	}
	if len(listByFullName) > 1 {
		return errors.Wrapf(ambigousResultsError(term), "%d matches found based on name", len(listByFullName))
	}

	// Find based on partial ID, returns true only if no duplicates
	if len(listByPartialID) == 1 {
		for _, v := range listByPartialID {
			return httputils.WriteJSON(w, http.StatusOK, v)
		}
	}
	if len(listByPartialID) > 1 {
		return errors.Wrapf(ambigousResultsError(term), "%d matches found based on ID prefix", len(listByPartialID))
	}

	return libnetwork.ErrNoSuchNetwork(term)
}

func (n *networkRouter) postNetworkCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var create types.NetworkCreateRequest
	if err := httputils.ReadJSON(r, &create); err != nil {
		return err
	}

	if nws, err := n.cluster.GetNetworksByName(create.Name); err == nil && len(nws) > 0 {
		return nameConflict(create.Name)
	}

	nw, err := n.backend.CreateNetwork(create)
	if err != nil {
		var warning string
		if _, ok := err.(libnetwork.NetworkNameError); ok {
			// check if user defined CheckDuplicate, if set true, return err
			// otherwise prepare a warning message
			if create.CheckDuplicate {
				return nameConflict(create.Name)
			}
			warning = libnetwork.NetworkNameError(create.Name).Error()
		}

		if _, ok := err.(libnetwork.ManagerRedirectError); !ok {
			return err
		}
		id, err := n.cluster.CreateNetwork(create)
		if err != nil {
			return err
		}
		nw = &types.NetworkCreateResponse{
			ID:      id,
			Warning: warning,
		}
	}

	return httputils.WriteJSON(w, http.StatusCreated, nw)
}

func (n *networkRouter) postNetworkConnect(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var connect types.NetworkConnect
	if err := httputils.ReadJSON(r, &connect); err != nil {
		return err
	}

	// Unlike other operations, we does not check ambiguity of the name/ID here.
	// The reason is that, In case of attachable network in swarm scope, the actual local network
	// may not be available at the time. At the same time, inside daemon `ConnectContainerToNetwork`
	// does the ambiguity check anyway. Therefore, passing the name to daemon would be enough.
	return n.backend.ConnectContainerToNetwork(connect.Container, vars["id"], connect.EndpointConfig)
}

func (n *networkRouter) postNetworkDisconnect(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	var disconnect types.NetworkDisconnect
	if err := httputils.ReadJSON(r, &disconnect); err != nil {
		return err
	}

	return n.backend.DisconnectContainerFromNetwork(disconnect.Container, vars["id"], disconnect.Force)
}

func (n *networkRouter) deleteNetwork(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	nw, err := n.findUniqueNetwork(vars["id"])
	if err != nil {
		return err
	}
	if nw.Scope == "swarm" {
		if err = n.cluster.RemoveNetwork(nw.ID); err != nil {
			return err
		}
	} else {
		if err := n.backend.DeleteNetwork(nw.ID); err != nil {
			return err
		}
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (n *networkRouter) postNetworksPrune(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	pruneFilters, err := filters.FromJSON(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	pruneReport, err := n.backend.NetworksPrune(ctx, pruneFilters)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, pruneReport)
}

// findUniqueNetwork will search network across different scopes (both local and swarm).
// NOTE: This findUniqueNetwork is different from FindNetwork in the daemon.
// In case multiple networks have duplicate names, return error.
// First find based on full ID, return immediately once one is found.
// If a network appears both in swarm and local, assume it is in local first
// For full name and partial ID, save the result first, and process later
// in case multiple records was found based on the same term
// TODO (yongtang): should we wrap with version here for backward compatibility?
func (n *networkRouter) findUniqueNetwork(term string) (types.NetworkResource, error) {
	listByFullName := map[string]types.NetworkResource{}
	listByPartialID := map[string]types.NetworkResource{}

	filter := filters.NewArgs(filters.Arg("idOrName", term))
	nw, _ := n.backend.GetNetworks(filter, types.NetworkListConfig{Detailed: true})
	for _, network := range nw {
		if network.ID == term {
			return network, nil
		}
		if network.Name == term && !network.Ingress {
			// No need to check the ID collision here as we are still in
			// local scope and the network ID is unique in this scope.
			listByFullName[network.ID] = network
		}
		if strings.HasPrefix(network.ID, term) {
			// No need to check the ID collision here as we are still in
			// local scope and the network ID is unique in this scope.
			listByPartialID[network.ID] = network
		}
	}

	nr, _ := n.cluster.GetNetworks(filter)
	for _, network := range nr {
		if network.ID == term {
			return network, nil
		}
		if network.Name == term {
			// Check the ID collision as we are in swarm scope here, and
			// the map (of the listByFullName) may have already had a
			// network with the same ID (from local scope previously)
			if _, ok := listByFullName[network.ID]; !ok {
				listByFullName[network.ID] = network
			}
		}
		if strings.HasPrefix(network.ID, term) {
			// Check the ID collision as we are in swarm scope here, and
			// the map (of the listByPartialID) may have already had a
			// network with the same ID (from local scope previously)
			if _, ok := listByPartialID[network.ID]; !ok {
				listByPartialID[network.ID] = network
			}
		}
	}

	// Find based on full name, returns true only if no duplicates
	if len(listByFullName) == 1 {
		for _, v := range listByFullName {
			return v, nil
		}
	}
	if len(listByFullName) > 1 {
		return types.NetworkResource{}, errdefs.InvalidParameter(errors.Errorf("network %s is ambiguous (%d matches found based on name)", term, len(listByFullName)))
	}

	// Find based on partial ID, returns true only if no duplicates
	if len(listByPartialID) == 1 {
		for _, v := range listByPartialID {
			return v, nil
		}
	}
	if len(listByPartialID) > 1 {
		return types.NetworkResource{}, errdefs.InvalidParameter(errors.Errorf("network %s is ambiguous (%d matches found based on ID prefix)", term, len(listByPartialID)))
	}

	return types.NetworkResource{}, errdefs.NotFound(libnetwork.ErrNoSuchNetwork(term))
}
