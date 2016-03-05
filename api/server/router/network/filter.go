package network

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/docker/docker/runconfig"
	"github.com/docker/engine-api/types/filters"
	"github.com/docker/libnetwork"
)

type filterHandler func([]libnetwork.Network, string) ([]libnetwork.Network, error)

var (
	// supportedFilters predefined some supported filter handler function
	supportedFilters = map[string]filterHandler{
		"type": filterNetworkByType,
		"name": filterNetworkByName,
		"id":   filterNetworkByID,
	}

	// acceptFilters is an acceptable filter flag list
	// generated for validation. e.g.
	// acceptedFilters = map[string]bool{
	//     "type": true,
	//     "name": true,
	//     "id":   true,
	// }
	acceptedFilters = func() map[string]bool {
		ret := make(map[string]bool)
		for k := range supportedFilters {
			ret[k] = true
		}
		return ret
	}()
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

func filterNetworkByName(nws []libnetwork.Network, name string) (retNws []libnetwork.Network, err error) {
	for _, nw := range nws {
		// exact match (fast path)
		if nw.Name() == name {
			retNws = append(retNws, nw)
			continue
		}

		// regexp match (slow path)
		match, err := regexp.MatchString(name, nw.Name())
		if err != nil || !match {
			continue
		} else {
			retNws = append(retNws, nw)
		}
	}
	return retNws, nil
}

func filterNetworkByID(nws []libnetwork.Network, id string) (retNws []libnetwork.Network, err error) {
	for _, nw := range nws {
		if strings.HasPrefix(nw.ID(), id) {
			retNws = append(retNws, nw)
		}
	}
	return retNws, nil
}

// filterAllNetworks filters network list according to user specified filter
// and returns user chosen networks
func filterNetworks(nws []libnetwork.Network, filter filters.Args) ([]libnetwork.Network, error) {
	// if filter is empty, return original network list
	if filter.Len() == 0 {
		return nws, nil
	}

	var displayNet []libnetwork.Network
	for fkey, fhandler := range supportedFilters {
		errFilter := filter.WalkValues(fkey, func(fval string) error {
			passList, err := fhandler(nws, fval)
			if err != nil {
				return err
			}
			displayNet = append(displayNet, passList...)
			return nil
		})
		if errFilter != nil {
			return nil, errFilter
		}
	}
	return displayNet, nil
}
