package constraint

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/docker/swarmkit/api"
)

const (
	eq = iota
	noteq

	nodeLabelPrefix   = "node.labels."
	engineLabelPrefix = "engine.labels."
)

var (
	alphaNumeric = regexp.MustCompile(`^(?i)[a-z_][a-z0-9\-_.]+$`)
	// value can be alphanumeric and some special characters. it shouldn't container
	// current or future operators like '>, <, ~', etc.
	valuePattern = regexp.MustCompile(`^(?i)[a-z0-9:\-_\s\.\*\(\)\?\+\[\]\\\^\$\|\/]+$`)

	// operators defines list of accepted operators
	operators = []string{"==", "!="}
)

// Constraint defines a constraint.
type Constraint struct {
	key      string
	operator int
	exp      string
}

// Parse parses list of constraints.
func Parse(env []string) ([]Constraint, error) {
	exprs := []Constraint{}
	for _, e := range env {
		found := false
		// each expr is in the form of "key op value"
		for i, op := range operators {
			if !strings.Contains(e, op) {
				continue
			}
			// split with the op
			parts := strings.SplitN(e, op, 2)

			if len(parts) < 2 {
				return nil, fmt.Errorf("invalid expr: %s", e)
			}

			part0 := strings.TrimSpace(parts[0])
			// validate key
			matched := alphaNumeric.MatchString(part0)
			if matched == false {
				return nil, fmt.Errorf("key '%s' is invalid", part0)
			}

			part1 := strings.TrimSpace(parts[1])

			// validate Value
			matched = valuePattern.MatchString(part1)
			if matched == false {
				return nil, fmt.Errorf("value '%s' is invalid", part1)
			}
			// TODO(dongluochen): revisit requirements to see if globing or regex are useful
			exprs = append(exprs, Constraint{key: part0, operator: i, exp: part1})

			found = true
			break // found an op, move to next entry
		}
		if !found {
			return nil, fmt.Errorf("constraint expected one operator from %s", strings.Join(operators, ", "))
		}
	}
	return exprs, nil
}

// Match checks if the Constraint matches the target strings.
func (c *Constraint) Match(whats ...string) bool {
	var match bool

	// full string match
	for _, what := range whats {
		// case insensitive compare
		if strings.EqualFold(c.exp, what) {
			match = true
			break
		}
	}

	switch c.operator {
	case eq:
		return match
	case noteq:
		return !match
	}

	return false
}

// NodeMatches returns true if the node satisfies the given constraints.
func NodeMatches(constraints []Constraint, n *api.Node) bool {
	for _, constraint := range constraints {
		switch {
		case strings.EqualFold(constraint.key, "node.id"):
			if !constraint.Match(n.ID) {
				return false
			}
		case strings.EqualFold(constraint.key, "node.hostname"):
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
		case strings.EqualFold(constraint.key, "node.ip"):
			nodeIP := net.ParseIP(n.Status.Addr)
			// single IP address, node.ip == 2001:db8::2
			if ip := net.ParseIP(constraint.exp); ip != nil {
				ipEq := ip.Equal(nodeIP)
				if (ipEq && constraint.operator != eq) || (!ipEq && constraint.operator == eq) {
					return false
				}
				continue
			}
			// CIDR subnet, node.ip != 210.8.4.0/24
			if _, subnet, err := net.ParseCIDR(constraint.exp); err == nil {
				within := subnet.Contains(nodeIP)
				if (within && constraint.operator != eq) || (!within && constraint.operator == eq) {
					return false
				}
				continue
			}
			// reject constraint with malformed address/network
			return false
		case strings.EqualFold(constraint.key, "node.role"):
			if !constraint.Match(n.Role.String()) {
				return false
			}
		case strings.EqualFold(constraint.key, "node.platform.os"):
			if n.Description == nil || n.Description.Platform == nil {
				if !constraint.Match("") {
					return false
				}
				continue
			}
			if !constraint.Match(n.Description.Platform.OS) {
				return false
			}
		case strings.EqualFold(constraint.key, "node.platform.arch"):
			if n.Description == nil || n.Description.Platform == nil {
				if !constraint.Match("") {
					return false
				}
				continue
			}
			if !constraint.Match(n.Description.Platform.Architecture) {
				return false
			}

		// node labels constraint in form like 'node.labels.key==value'
		case len(constraint.key) > len(nodeLabelPrefix) && strings.EqualFold(constraint.key[:len(nodeLabelPrefix)], nodeLabelPrefix):
			if n.Spec.Annotations.Labels == nil {
				if !constraint.Match("") {
					return false
				}
				continue
			}
			label := constraint.key[len(nodeLabelPrefix):]
			// label itself is case sensitive
			val := n.Spec.Annotations.Labels[label]
			if !constraint.Match(val) {
				return false
			}

		// engine labels constraint in form like 'engine.labels.key!=value'
		case len(constraint.key) > len(engineLabelPrefix) && strings.EqualFold(constraint.key[:len(engineLabelPrefix)], engineLabelPrefix):
			if n.Description == nil || n.Description.Engine == nil || n.Description.Engine.Labels == nil {
				if !constraint.Match("") {
					return false
				}
				continue
			}
			label := constraint.key[len(engineLabelPrefix):]
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
