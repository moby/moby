package scheduler

import (
	"strings"

	"github.com/docker/swarmkit/api"
)

// ConstraintFilter selects only nodes that match certain labels.
type ConstraintFilter struct {
	constraints []Expr
}

// SetTask returns true when the filter is enable for a given task.
func (f *ConstraintFilter) SetTask(t *api.Task) bool {
	if t.Spec.Placement != nil && len(t.Spec.Placement.Constraints) > 0 {
		constraints, err := ParseExprs(t.Spec.Placement.Constraints)
		if err == nil {
			f.constraints = constraints
			return true
		}
	}
	return false
}

// Check returns true if the task's constraint is supported by the given node.
func (f *ConstraintFilter) Check(n *NodeInfo) bool {
	for _, constraint := range f.constraints {
		switch constraint.Key {
		case "node.id":
			if !constraint.Match(n.ID) {
				return false
			}
		case "node.name":
			// if this node doesn't have hostname
			// it's equivalent to match an empty hostname
			// where '==' would fail, '!=' matches
			if n.Description == nil {
				if !constraint.Match("") {
					return false
				}
				continue
			}
			if !constraint.Match(n.Description.Hostname) {
				return false
			}
		default:
			// default is node label in form like 'node.labels.key==value'
			// if it is not well formed, always fails it
			if !strings.HasPrefix(constraint.Key, "node.labels.") {
				return false
			}
			// if the node doesn't have any label,
			// it's equivalent to match an empty value.
			// that is, 'node.labels.key!=value' should pass and
			// 'node.labels.key==value' should fail
			if n.Spec.Annotations.Labels == nil {
				if !constraint.Match("") {
					return false
				}
				continue
			}
			label := constraint.Key[len("node.labels."):]
			// if the node doesn't have this specific label,
			// val is an empty string
			val := n.Spec.Annotations.Labels[label]
			if !constraint.Match(val) {
				return false
			}
		}
	}

	return true
}
