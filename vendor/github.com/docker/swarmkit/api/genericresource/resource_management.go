package genericresource

import (
	"fmt"
	"github.com/docker/swarmkit/api"
)

// Claim assigns GenericResources to a task by taking them from the
// node's GenericResource list and storing them in the task's available list
func Claim(nodeAvailableResources, taskAssigned *[]*api.GenericResource,
	taskReservations []*api.GenericResource) error {
	var resSelected []*api.GenericResource

	for _, res := range taskReservations {
		tr := res.GetDiscreteResourceSpec()
		if tr == nil {
			return fmt.Errorf("task should only hold Discrete type")
		}

		// Select the resources
		nrs, err := selectNodeResources(*nodeAvailableResources, tr)
		if err != nil {
			return err
		}

		resSelected = append(resSelected, nrs...)
	}

	ClaimResources(nodeAvailableResources, taskAssigned, resSelected)
	return nil
}

// ClaimResources adds the specified resources to the task's list
// and removes them from the node's generic resource list
func ClaimResources(nodeAvailableResources, taskAssigned *[]*api.GenericResource,
	resSelected []*api.GenericResource) {
	*taskAssigned = append(*taskAssigned, resSelected...)
	ConsumeNodeResources(nodeAvailableResources, resSelected)
}

func selectNodeResources(nodeRes []*api.GenericResource,
	tr *api.DiscreteGenericResource) ([]*api.GenericResource, error) {
	var nrs []*api.GenericResource

	for _, res := range nodeRes {
		if Kind(res) != tr.Kind {
			continue
		}

		switch nr := res.Resource.(type) {
		case *api.GenericResource_DiscreteResourceSpec:
			if nr.DiscreteResourceSpec.Value >= tr.Value && tr.Value != 0 {
				nrs = append(nrs, NewDiscrete(tr.Kind, tr.Value))
			}

			return nrs, nil
		case *api.GenericResource_NamedResourceSpec:
			nrs = append(nrs, res.Copy())

			if int64(len(nrs)) == tr.Value {
				return nrs, nil
			}
		}
	}

	if len(nrs) == 0 {
		return nil, fmt.Errorf("not enough resources available for task reservations: %+v", tr)
	}

	return nrs, nil
}

// Reclaim adds the resources taken by the task to the node's store
func Reclaim(nodeAvailableResources *[]*api.GenericResource, taskAssigned, nodeRes []*api.GenericResource) error {
	err := reclaimResources(nodeAvailableResources, taskAssigned)
	if err != nil {
		return err
	}

	sanitize(nodeRes, nodeAvailableResources)

	return nil
}

func reclaimResources(nodeAvailableResources *[]*api.GenericResource, taskAssigned []*api.GenericResource) error {
	// The node could have been updated
	if nodeAvailableResources == nil {
		return fmt.Errorf("node no longer has any resources")
	}

	for _, res := range taskAssigned {
		switch tr := res.Resource.(type) {
		case *api.GenericResource_DiscreteResourceSpec:
			nrs := GetResource(tr.DiscreteResourceSpec.Kind, *nodeAvailableResources)

			// If the resource went down to 0 it's no longer in the
			// available list
			if len(nrs) == 0 {
				*nodeAvailableResources = append(*nodeAvailableResources, res.Copy())
			}

			if len(nrs) != 1 {
				continue // Type change
			}

			nr := nrs[0].GetDiscreteResourceSpec()
			if nr == nil {
				continue // Type change
			}

			nr.Value += tr.DiscreteResourceSpec.Value
		case *api.GenericResource_NamedResourceSpec:
			*nodeAvailableResources = append(*nodeAvailableResources, res.Copy())
		}
	}

	return nil
}

// sanitize checks that nodeAvailableResources does not add resources unknown
// to the nodeSpec (nodeRes) or goes over the integer bound specified
// by the spec.
// Note this is because the user is able to update a node's resources
func sanitize(nodeRes []*api.GenericResource, nodeAvailableResources *[]*api.GenericResource) {
	// - We add the sanitized resources at the end, after
	// having removed the elements from the list

	// - When a set changes to a Discrete we also need
	// to make sure that we don't add the Discrete multiple
	// time hence, the need of a map to remember that
	var sanitized []*api.GenericResource
	kindSanitized := make(map[string]struct{})
	w := 0

	for _, na := range *nodeAvailableResources {
		ok, nrs := sanitizeResource(nodeRes, na)
		if !ok {
			if _, ok = kindSanitized[Kind(na)]; ok {
				continue
			}

			kindSanitized[Kind(na)] = struct{}{}
			sanitized = append(sanitized, nrs...)

			continue
		}

		(*nodeAvailableResources)[w] = na
		w++
	}

	*nodeAvailableResources = (*nodeAvailableResources)[:w]
	*nodeAvailableResources = append(*nodeAvailableResources, sanitized...)
}

// Returns true if the element is in nodeRes and "sane"
// Returns false if the element isn't in nodeRes and "sane" and the element(s) that should be replacing it
func sanitizeResource(nodeRes []*api.GenericResource, res *api.GenericResource) (ok bool, nrs []*api.GenericResource) {
	switch na := res.Resource.(type) {
	case *api.GenericResource_DiscreteResourceSpec:
		nrs := GetResource(na.DiscreteResourceSpec.Kind, nodeRes)

		// Type change or removed: reset
		if len(nrs) != 1 {
			return false, nrs
		}

		// Type change: reset
		nr := nrs[0].GetDiscreteResourceSpec()
		if nr == nil {
			return false, nrs
		}

		// Amount change: reset
		if na.DiscreteResourceSpec.Value > nr.Value {
			return false, nrs
		}
	case *api.GenericResource_NamedResourceSpec:
		nrs := GetResource(na.NamedResourceSpec.Kind, nodeRes)

		// Type change
		if len(nrs) == 0 {
			return false, nrs
		}

		for _, nr := range nrs {
			// Type change: reset
			if nr.GetDiscreteResourceSpec() != nil {
				return false, nrs
			}

			if na.NamedResourceSpec.Value == nr.GetNamedResourceSpec().Value {
				return true, nil
			}
		}

		// Removed
		return false, nil
	}

	return true, nil
}
