package scheduler

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	eq = iota
	noteq
)

var (
	alphaNumeric = regexp.MustCompile(`^(?i)[a-z_][a-z0-9\-_.]+$`)
	// value can be alphanumeric and some special characters. it shouldn't container
	// current or future operators like '>, <, ~', etc.
	valuePattern = regexp.MustCompile(`^(?i)[a-z0-9:\-_\s\.\*\(\)\?\+\[\]\\\^\$\|\/]+$`)

	// operators defines list of accepted operators
	operators = []string{"==", "!="}
)

// Expr defines a constraint
type Expr struct {
	Key      string
	operator int
	exp      string
}

// ParseExprs parses list of constraints into Expr list
func ParseExprs(env []string) ([]Expr, error) {
	exprs := []Expr{}
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
			// validate Key
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
			exprs = append(exprs, Expr{Key: part0, operator: i, exp: part1})

			found = true
			break // found an op, move to next entry
		}
		if !found {
			return nil, fmt.Errorf("constraint expected one operator from %s", strings.Join(operators, ", "))
		}
	}
	return exprs, nil
}

// Match checks if the Expr matches the target strings.
func (e *Expr) Match(whats ...string) bool {
	var match bool

	// full string match
	for _, what := range whats {
		if e.exp == what {
			match = true
			break
		}
	}

	switch e.operator {
	case eq:
		return match
	case noteq:
		return !match
	}

	return false
}
