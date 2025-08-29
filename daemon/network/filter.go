package network

import (
	"time"

	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/v2/daemon/internal/timestamp"
	"github.com/moby/moby/v2/errdefs"
	"github.com/pkg/errors"
)

var acceptedFilters = map[string]bool{
	"dangling": true,
	"driver":   true,
	"id":       true,
	"label":    true,
	"label!":   true,
	"name":     true,
	"scope":    true,
	"type":     true,
	"until":    true,
}

type Filter struct {
	args filters.Args

	filterByUse, danglingOnly bool
	until                     time.Time

	// IDAlsoMatchesName makes the "id" filter term also match against
	// network names.
	IDAlsoMatchesName bool
}

type FilterNetwork interface {
	Driver() string
	Name() string
	ID() string
	Labels() map[string]string
	Scope() string
	Created() time.Time
	ContainerAttachments() int
	ServiceAttachments() int
}

// NewFilter returns a network filter that filters by the provided args.
//
// An [errdefs.InvalidParameter] error is returned if the filter args are not
// well-formed.
func NewFilter(args filters.Args) (Filter, error) {
	if err := args.Validate(acceptedFilters); err != nil {
		return Filter{}, err
	}
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
	until := time.Time{}
	if untilFilters := args.Get("until"); len(untilFilters) > 0 {
		if len(untilFilters) > 1 {
			return Filter{}, errdefs.InvalidParameter(errors.New("more than one until filter specified"))
		}
		ts, err := timestamp.GetTimestamp(untilFilters[0], time.Now())
		if err != nil {
			return Filter{}, errdefs.InvalidParameter(err)
		}
		seconds, nanoseconds, err := timestamp.ParseTimestamps(ts, 0)
		if err != nil {
			return Filter{}, errdefs.InvalidParameter(err)
		}
		until = time.Unix(seconds, nanoseconds)
	}
	return Filter{
		args:         args,
		filterByUse:  filterByUse,
		danglingOnly: danglingOnly,
		until:        until,
	}, nil
}

func (f Filter) IsZero() bool {
	return f.args.Len() == 0
}

func (f Filter) Get(key string) []string {
	return f.args.Get(key)
}

// Matches returns true if nw satisfies the filter criteria.
func (f Filter) Matches(nw FilterNetwork) bool {
	if f.args.Len() == 0 {
		return true
	}

	if f.args.Contains("driver") &&
		!f.args.ExactMatch("driver", nw.Driver()) {
		return false
	}
	if f.args.Contains("name") &&
		!f.args.Match("name", nw.Name()) {
		return false
	}
	if f.args.Contains("id") &&
		!f.args.Match("id", nw.ID()) &&
		(!f.IDAlsoMatchesName || !f.args.Match("id", nw.Name())) {
		return false
	}
	if f.args.Contains("label") &&
		!f.args.MatchKVList("label", nw.Labels()) {
		return false
	}
	if f.args.Contains("label!") &&
		f.args.MatchKVList("label!", nw.Labels()) {
		return false
	}
	if f.args.Contains("scope") &&
		!f.args.ExactMatch("scope", nw.Scope()) {
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
	if !f.until.IsZero() &&
		nw.Created().After(f.until) {
		return false
	}
	return true
}

func matchesUse(danglingOnly bool, nw FilterNetwork) bool {
	if danglingOnly {
		return !IsPredefined(nw.Name()) && nw.ContainerAttachments() == 0 && nw.ServiceAttachments() == 0
	}
	return IsPredefined(nw.Name()) || nw.ContainerAttachments() > 0 || nw.ServiceAttachments() > 0
}

func validateNetworkTypeFilter(netType string) error {
	switch netType {
	case "builtin", "custom":
		return nil
	default:
		return errors.Errorf("invalid filter: 'type'='%s'", netType)
	}
}

func matchesType(netTypes []string, nw FilterNetwork) bool {
	for _, netType := range netTypes {
		switch netType {
		case "builtin":
			if IsPredefined(nw.Name()) {
				return true
			}
		case "custom":
			if !IsPredefined(nw.Name()) {
				return true
			}
		}
	}
	return false
}
