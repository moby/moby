package network // import "github.com/docker/docker/daemon/network"

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/runconfig"
	"github.com/pkg/errors"
)

// FilterNetworks filters network list according to user specified filter
// and returns user chosen networks
func FilterNetworks(nws []types.NetworkResource, filter filters.Args) ([]types.NetworkResource, error) {
	// if filter is empty, return original network list
	if filter.Len() == 0 {
		return nws, nil
	}

	displayNet := nws[:0]
	for _, nw := range nws {
		if filter.Contains("driver") {
			if !filter.ExactMatch("driver", nw.Driver) {
				continue
			}
		}
		if filter.Contains("name") {
			if !filter.Match("name", nw.Name) {
				continue
			}
		}
		if filter.Contains("id") {
			if !filter.Match("id", nw.ID) {
				continue
			}
		}
		if filter.Contains("label") {
			if !filter.MatchKVList("label", nw.Labels) {
				continue
			}
		}
		if filter.Contains("scope") {
			if !filter.ExactMatch("scope", nw.Scope) {
				continue
			}
		}

		if filter.Contains("idOrName") {
			if !filter.Match("name", nw.Name) && !filter.Match("id", nw.Name) {
				continue
			}
		}
		displayNet = append(displayNet, nw)
	}

	if values := filter.Get("dangling"); len(values) > 0 {
		if len(values) > 1 {
			return nil, errdefs.InvalidParameter(errors.New(`got more than one value for filter key "dangling"`))
		}

		var danglingOnly bool
		switch values[0] {
		case "0", "false":
			// dangling is false already
		case "1", "true":
			danglingOnly = true
		default:
			return nil, errdefs.InvalidParameter(errors.New(`invalid value for filter 'dangling', must be "true" (or "1"), or "false" (or "0")`))
		}

		displayNet = filterNetworkByUse(displayNet, danglingOnly)
	}

	if filter.Contains("type") {
		typeNet := []types.NetworkResource{}
		errFilter := filter.WalkValues("type", func(fval string) error {
			passList, err := filterNetworkByType(displayNet, fval)
			if err != nil {
				return err
			}
			typeNet = append(typeNet, passList...)
			return nil
		})
		if errFilter != nil {
			return nil, errFilter
		}
		displayNet = typeNet
	}

	return displayNet, nil
}

func filterNetworkByUse(nws []types.NetworkResource, danglingOnly bool) []types.NetworkResource {
	retNws := []types.NetworkResource{}

	filterFunc := func(nw types.NetworkResource) bool {
		if danglingOnly {
			return !runconfig.IsPreDefinedNetwork(nw.Name) && len(nw.Containers) == 0 && len(nw.Services) == 0
		}
		return runconfig.IsPreDefinedNetwork(nw.Name) || len(nw.Containers) > 0 || len(nw.Services) > 0
	}

	for _, nw := range nws {
		if filterFunc(nw) {
			retNws = append(retNws, nw)
		}
	}

	return retNws
}

func filterNetworkByType(nws []types.NetworkResource, netType string) ([]types.NetworkResource, error) {
	retNws := []types.NetworkResource{}
	switch netType {
	case "builtin":
		for _, nw := range nws {
			if runconfig.IsPreDefinedNetwork(nw.Name) {
				retNws = append(retNws, nw)
			}
		}
	case "custom":
		for _, nw := range nws {
			if !runconfig.IsPreDefinedNetwork(nw.Name) {
				retNws = append(retNws, nw)
			}
		}
	default:
		return nil, errors.Errorf("invalid filter: 'type'='%s'", netType)
	}
	return retNws, nil
}
