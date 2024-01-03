package genericresource

import (
	"fmt"

	"github.com/moby/swarmkit/v2/api"
)

// ValidateTask validates that the task only uses integers
// for generic resources
func ValidateTask(resources *api.Resources) error {
	for _, v := range resources.Generic {
		if v.GetDiscreteResourceSpec() != nil {
			continue
		}

		return fmt.Errorf("invalid argument for resource %s", Kind(v))
	}

	return nil
}

// HasEnough returns true if node can satisfy the task's GenericResource request
func HasEnough(nodeRes []*api.GenericResource, taskRes *api.GenericResource) (bool, error) {
	t := taskRes.GetDiscreteResourceSpec()
	if t == nil {
		return false, fmt.Errorf("task should only hold Discrete type")
	}

	if nodeRes == nil {
		return false, nil
	}

	nrs := GetResource(t.Kind, nodeRes)
	if len(nrs) == 0 {
		return false, nil
	}

	switch nr := nrs[0].Resource.(type) {
	case *api.GenericResource_DiscreteResourceSpec:
		if t.Value > nr.DiscreteResourceSpec.Value {
			return false, nil
		}
	case *api.GenericResource_NamedResourceSpec:
		if t.Value > int64(len(nrs)) {
			return false, nil
		}
	}

	return true, nil
}

// HasResource checks if there is enough "res" in the "resources" argument
func HasResource(res *api.GenericResource, resources []*api.GenericResource) bool {
	for _, r := range resources {
		if Kind(res) != Kind(r) {
			continue
		}

		switch rtype := r.Resource.(type) {
		case *api.GenericResource_DiscreteResourceSpec:
			if res.GetDiscreteResourceSpec() == nil {
				return false
			}

			if res.GetDiscreteResourceSpec().Value < rtype.DiscreteResourceSpec.Value {
				return false
			}

			return true
		case *api.GenericResource_NamedResourceSpec:
			if res.GetNamedResourceSpec() == nil {
				return false
			}

			if res.GetNamedResourceSpec().Value != rtype.NamedResourceSpec.Value {
				continue
			}

			return true
		}
	}

	return false
}
