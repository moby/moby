package scheduler

import (
	"strings"

	"github.com/docker/swarmkit/api"
)

const (
	nodeLabelPrefix   = "node.labels."
	engineLabelPrefix = "engine.labels."
)

// ConstraintFilter selects only nodes that match certain labels.
type ConstraintFilter struct {
	constraints []Expr
}

// SetTask returns true when the filter is enable for a given task.
func (f *ConstraintFilter) SetTask(t *api.Task) bool {
	if t.Spec.Placement == nil || len(t.Spec.Placement.Constraints) == 0 {
		return false
	}

	constraints, err := ParseExprs(t.Spec.Placement.Constraints)
	if err != nil {
		// constraints have been validated at controlapi
		// if in any case it finds an error here, treat this task
		// as constraint filter disabled.
		return false
	}
	f.constraints = constraints
	return true
}

// Check returns true if the task's constraint is supported by the given node.
func (f *ConstraintFilter) Check(n *NodeInfo) bool {
	for _, constraint := range f.constraints {
		switch {
		case strings.EqualFold(constraint.Key, "node.id"):
			if !constraint.Match(n.ID) {
				return false
			}
		case strings.EqualFold(constraint.Key, "node.hostname"):
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
		case strings.EqualFold(constraint.Key, "node.role"):
			if !constraint.Match(n.Spec.Role.String()) {
				return false
			}

		// node labels constraint in form like 'node.labels.key==value'
		case len(constraint.Key) > len(nodeLabelPrefix) && strings.EqualFold(constraint.Key[:len(nodeLabelPrefix)], nodeLabelPrefix):
			if n.Spec.Annotations.Labels == nil {
				if !constraint.Match("") {
					return false
				}
				continue
			}
			label := constraint.Key[len(nodeLabelPrefix):]
			// label itself is case sensitive
			val := n.Spec.Annotations.Labels[label]
			if !constraint.Match(val) {
				return false
			}

		// engine labels constraint in form like 'engine.labels.key!=value'
		case len(constraint.Key) > len(engineLabelPrefix) && strings.EqualFold(constraint.Key[:len(engineLabelPrefix)], engineLabelPrefix):
			if n.Description == nil || n.Description.Engine == nil || n.Description.Engine.Labels == nil {
				if !constraint.Match("") {
					return false
				}
				continue
			}
			label := constraint.Key[len(engineLabelPrefix):]
			val := n.Description.Engine.Labels[label]
			if !constraint.Match(val) {
				return false
			}
		default:
			// key doesn't match predefined syntax
			return false
		}
	}

	return true
}
