// Package checker provide Docker specific implementations of the go-check.Checker interface.
package checker

import (
	"fmt"
	"strings"

	"github.com/go-check/check"
)

// As a commodity, we bring all check.Checker variables into the current namespace to avoid having
// to think about check.X versus checker.X.
var (
	DeepEquals   = check.DeepEquals
	Equals       = check.Equals
	ErrorMatches = check.ErrorMatches
	FitsTypeOf   = check.FitsTypeOf
	HasLen       = check.HasLen
	Implements   = check.Implements
	IsNil        = check.IsNil
	Matches      = check.Matches
	Not          = check.Not
	NotNil       = check.NotNil
	PanicMatches = check.PanicMatches
	Panics       = check.Panics
)

// Contains checker verifies that string value contains a substring.
var Contains check.Checker = &containsChecker{
	&check.CheckerInfo{
		Name:   "Contains",
		Params: []string{"value", "substring"},
	},
}

type containsChecker struct {
	*check.CheckerInfo
}

func (checker *containsChecker) Check(params []interface{}, names []string) (bool, string) {
	return contains(params[0], params[1])
}

func contains(value, substring interface{}) (bool, string) {
	substringStr, ok := substring.(string)
	if !ok {
		return false, "Substring must be a string"
	}
	valueStr, valueIsStr := value.(string)
	if !valueIsStr {
		if valueWithStr, valueHasStr := value.(fmt.Stringer); valueHasStr {
			valueStr, valueIsStr = valueWithStr.String(), true
		}
	}
	if valueIsStr {
		return strings.Contains(valueStr, substringStr), ""
	}
	return false, "Obtained value is not a string and has no .String()"
}
