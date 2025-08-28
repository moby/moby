package network

import (
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/errdefs"
	"github.com/pkg/errors"
)

type Filter struct {
	args filters.Args

	filterByUse, danglingOnly bool

	// IDAlsoMatchesName makes the "id" filter term also match against
	// network names.
	IDAlsoMatchesName bool
}

// NewFilter returns a network filter that filters by the provided args.
//
// An [errdefs.InvalidParameter] error is returned if the filter args are not
// well-formed.
func NewFilter(args filters.Args) (Filter, error) {
	var filterByUse, danglingOnly bool
	if values := args.Get("dangling"); len(values) > 0 {
		if len(values) > 1 {
			return Filter{}, errdefs.InvalidParameter(errors.New(`got more than one value for filter key "dangling"`))
		}

		filterByUse = true
		switch values[0] {
		case "0", "false":
			// dangling is false already
		case "1", "true":
			danglingOnly = true
		default:
			return Filter{}, errdefs.InvalidParameter(errors.New(`invalid value for filter 'dangling', must be "true" (or "1"), or "false" (or "0")`))
		}
	}
	if err := args.WalkValues("type", validateNetworkTypeFilter); err != nil {
		return Filter{}, err
	}
	return Filter{args: args, filterByUse: filterByUse, danglingOnly: danglingOnly}, nil
}

// Matches returns true if nw satisfies the filter criteria.
func (f Filter) Matches(nw network.Summary) bool {
	if f.args.Len() == 0 {
		return true
	}

	if f.args.Contains("driver") &&
		!f.args.ExactMatch("driver", nw.Driver) {
		return false
	}
	if f.args.Contains("name") &&
		!f.args.Match("name", nw.Name) {
		return false
	}
	if f.args.Contains("id") &&
		!f.args.Match("id", nw.ID) &&
		(!f.IDAlsoMatchesName || !f.args.Match("id", nw.Name)) {
		return false
	}
	if f.args.Contains("label") &&
		!f.args.MatchKVList("label", nw.Labels) {
		return false
	}
	if f.args.Contains("scope") &&
		!f.args.ExactMatch("scope", nw.Scope) {
		return false
	}
	if f.filterByUse &&
		!matchesUse(f.danglingOnly, nw) {
		return false
	}
	if netTypes := f.args.Get("type"); len(netTypes) > 0 &&
		!matchesType(netTypes, nw) {
		return false
	}
	return true
}

func matchesUse(danglingOnly bool, nw network.Summary) bool {
	if danglingOnly {
		return !IsPredefined(nw.Name) && len(nw.Containers) == 0 && len(nw.Services) == 0
	}
	return IsPredefined(nw.Name) || len(nw.Containers) > 0 || len(nw.Services) > 0
}

func validateNetworkTypeFilter(netType string) error {
	switch netType {
	case "builtin", "custom":
		return nil
	default:
		return errors.Errorf("invalid filter: 'type'='%s'", netType)
	}
}

func matchesType(netTypes []string, nw network.Summary) bool {
	var matched bool
	for _, netType := range netTypes {
		switch netType {
		case "builtin":
			matched = IsPredefined(nw.Name)
		case "custom":
			matched = !IsPredefined(nw.Name)
		}
		if matched {
			break
		}
	}
	return matched
}
