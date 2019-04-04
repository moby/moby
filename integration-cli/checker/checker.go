// Package checker provides Docker specific implementations of the go-check.Checker interface.
package checker // import "github.com/docker/docker/integration-cli/checker"

import (
	"github.com/go-check/check"
	"github.com/vdemeester/shakers"
)

// As a commodity, we bring all check.Checker variables into the current namespace to avoid having
// to think about check.X versus checker.X.
var (
	DeepEquals = check.DeepEquals
	HasLen     = check.HasLen
	IsNil      = check.IsNil
	Matches    = check.Matches
	Not        = check.Not
	NotNil     = check.NotNil

	Contains           = shakers.Contains
	Count              = shakers.Count
	Equals             = shakers.Equals
	False              = shakers.False
	GreaterOrEqualThan = shakers.GreaterOrEqualThan
	GreaterThan        = shakers.GreaterThan
	HasPrefix          = shakers.HasPrefix
	LessOrEqualThan    = shakers.LessOrEqualThan
	True               = shakers.True
)
