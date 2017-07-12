package genericresource

import (
	"github.com/docker/swarmkit/api"
)

// NewSet creates a set object
func NewSet(key string, vals ...string) []*api.GenericResource {
	rs := make([]*api.GenericResource, 0, len(vals))

	for _, v := range vals {
		rs = append(rs, NewString(key, v))
	}

	return rs
}

// NewString creates a String resource
func NewString(key, val string) *api.GenericResource {
	return &api.GenericResource{
		Resource: &api.GenericResource_NamedResourceSpec{
			NamedResourceSpec: &api.NamedGenericResource{
				Kind:  key,
				Value: val,
			},
		},
	}
}

// NewDiscrete creates a Discrete resource
func NewDiscrete(key string, val int64) *api.GenericResource {
	return &api.GenericResource{
		Resource: &api.GenericResource_DiscreteResourceSpec{
			DiscreteResourceSpec: &api.DiscreteGenericResource{
				Kind:  key,
				Value: val,
			},
		},
	}
}

// GetResource returns resources from the "resources" parameter matching the kind key
func GetResource(kind string, resources []*api.GenericResource) []*api.GenericResource {
	var res []*api.GenericResource

	for _, r := range resources {
		if Kind(r) != kind {
			continue
		}

		res = append(res, r)
	}

	return res
}

// ConsumeNodeResources removes "res" from nodeAvailableResources
func ConsumeNodeResources(nodeAvailableResources *[]*api.GenericResource, res []*api.GenericResource) {
	if nodeAvailableResources == nil {
		return
	}

	w := 0

loop:
	for _, na := range *nodeAvailableResources {
		for _, r := range res {
			if Kind(na) != Kind(r) {
				continue
			}

			if remove(na, r) {
				continue loop
			}
			// If this wasn't the right element then
			// we need to continue
		}

		(*nodeAvailableResources)[w] = na
		w++
	}

	*nodeAvailableResources = (*nodeAvailableResources)[:w]
}

// Returns true if the element is to be removed from the list
func remove(na, r *api.GenericResource) bool {
	switch tr := r.Resource.(type) {
	case *api.GenericResource_DiscreteResourceSpec:
		if na.GetDiscreteResourceSpec() == nil {
			return false // Type change, ignore
		}

		na.GetDiscreteResourceSpec().Value -= tr.DiscreteResourceSpec.Value
		if na.GetDiscreteResourceSpec().Value <= 0 {
			return true
		}
	case *api.GenericResource_NamedResourceSpec:
		if na.GetNamedResourceSpec() == nil {
			return false // Type change, ignore
		}

		if tr.NamedResourceSpec.Value != na.GetNamedResourceSpec().Value {
			return false // not the right item, ignore
		}

		return true
	}

	return false
}
