package genericresource

import (
	"fmt"
	"github.com/docker/swarmkit/api"
)

// ValidateTask validates that the task only uses integers
// for generic resources
func ValidateTask(resources *api.Resources) error {
	for _, v := range resources.Generic {
		if v.GetDiscrete() != nil {
			continue
		}

		return fmt.Errorf("invalid argument for resource %s", Kind(v))
	}

	return nil
}

// HasEnough returns true if node can satisfy the task's GenericResource request
func HasEnough(nodeRes []*api.GenericResource, taskRes *api.GenericResource) (bool, error) {
	t := taskRes.GetDiscrete()
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
	case *api.GenericResource_Discrete:
		if t.Value > nr.Discrete.Value {
			return false, nil
		}
	case *api.GenericResource_Str:
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
		case *api.GenericResource_Discrete:
			if res.GetDiscrete() == nil {
				return false
			}

			if res.GetDiscrete().Value < rtype.Discrete.Value {
				return false
			}

			return true
		case *api.GenericResource_Str:
			if res.GetStr() == nil {
				return false
			}

			if res.GetStr().Value != rtype.Str.Value {
				continue
			}

			return true
		}
	}

	return false
}
