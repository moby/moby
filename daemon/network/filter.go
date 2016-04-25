package network

import (
	"fmt"

	"github.com/docker/docker/runconfig"
	"github.com/docker/engine-api/types/filters"
	"github.com/docker/libnetwork"
)

type filterHandler func([]libnetwork.Network, string) ([]libnetwork.Network, error)

var (
	// AcceptedFilters is an acceptable filters for validation
	AcceptedFilters = map[string]bool{
		"driver": true,
		"type":   true,
		"name":   true,
		"id":     true,
		"label":  true,
	}
)

func filterNetworkByType(nws []libnetwork.Network, netType string) (retNws []libnetwork.Network, err error) {
	switch netType {
	case "builtin":
		for _, nw := range nws {
			if runconfig.IsPreDefinedNetwork(nw.Name()) {
				retNws = append(retNws, nw)
			}
		}
	case "custom":
		for _, nw := range nws {
			if !runconfig.IsPreDefinedNetwork(nw.Name()) {
				retNws = append(retNws, nw)
			}
		}
	default:
		return nil, fmt.Errorf("Invalid filter: 'type'='%s'", netType)
	}
	return retNws, nil
}

// FilterNetworks filters network list according to user specified filter
// and returns user chosen networks
func FilterNetworks(nws []libnetwork.Network, filter filters.Args) ([]libnetwork.Network, error) {
	// if filter is empty, return original network list
	if filter.Len() == 0 {
		return nws, nil
	}

	var displayNet []libnetwork.Network
	for _, nw := range nws {
		if filter.Include("driver") {
			if !filter.ExactMatch("driver", nw.Type()) {
				continue
			}
		}
		if filter.Include("name") {
			if !filter.Match("name", nw.Name()) {
				continue
			}
		}
		if filter.Include("id") {
			if !filter.Match("id", nw.ID()) {
				continue
			}
		}
		if filter.Include("label") {
			if !filter.MatchKVList("label", nw.Info().Labels()) {
				continue
			}
		}
		displayNet = append(displayNet, nw)
	}

	if filter.Include("type") {
		var typeNet []libnetwork.Network
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
