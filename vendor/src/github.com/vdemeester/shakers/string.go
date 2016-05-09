// Package shakers provide some checker implementation the go-check.Checker interface.
package shakers

import (
	"fmt"
	"strings"

	"github.com/go-check/check"
)

// Contains checker verifies that obtained value contains a substring.
var Contains check.Checker = &substringChecker{
	&check.CheckerInfo{
		Name:   "Contains",
		Params: []string{"obtained", "substring"},
	},
	strings.Contains,
}

// ContainsAny checker verifies that any Unicode code points in chars
// are in the obtained string.
var ContainsAny check.Checker = &substringChecker{
	&check.CheckerInfo{
		Name:   "ContainsAny",
		Params: []string{"obtained", "chars"},
	},
	strings.ContainsAny,
}

// HasPrefix checker verifies that obtained value has the specified substring as prefix
var HasPrefix check.Checker = &substringChecker{
	&check.CheckerInfo{
		Name:   "HasPrefix",
		Params: []string{"obtained", "prefix"},
	},
	strings.HasPrefix,
}

// HasSuffix checker verifies that obtained value has the specified substring as prefix
var HasSuffix check.Checker = &substringChecker{
	&check.CheckerInfo{
		Name:   "HasSuffix",
		Params: []string{"obtained", "suffix"},
	},
	strings.HasSuffix,
}

// EqualFold checker verifies that obtained value is, interpreted as UTF-8 strings, are equal under Unicode case-folding.
var EqualFold check.Checker = &substringChecker{
	&check.CheckerInfo{
		Name:   "EqualFold",
		Params: []string{"obtained", "expected"},
	},
	strings.EqualFold,
}

type substringChecker struct {
	*check.CheckerInfo
	substringFunction func(string, string) bool
}

func (checker *substringChecker) Check(params []interface{}, names []string) (bool, string) {
	obtained := params[0]
	substring := params[1]
	substringStr, ok := substring.(string)
	if !ok {
		return false, fmt.Sprintf("%s value must be a string.", names[1])
	}
	obtainedString, obtainedIsStr := obtained.(string)
	if !obtainedIsStr {
		if obtainedWithStringer, obtainedHasStringer := obtained.(fmt.Stringer); obtainedHasStringer {
			obtainedString, obtainedIsStr = obtainedWithStringer.String(), true
		}
	}
	if obtainedIsStr {
		return checker.substringFunction(obtainedString, substringStr), ""
	}
	return false, "obtained value is not a string and has no .String()."
}

// IndexAny checker verifies that the index of the first instance of any Unicode code point from chars in the obtained value is equal to expected
var IndexAny check.Checker = &substringCountChecker{
	&check.CheckerInfo{
		Name:   "IndexAny",
		Params: []string{"obtained", "chars", "expected"},
	},
	strings.IndexAny,
}

// Index checker verifies that the index of the first instance of sep in the obtained value is equal to expected
var Index check.Checker = &substringCountChecker{
	&check.CheckerInfo{
		Name:   "Index",
		Params: []string{"obtained", "sep", "expected"},
	},
	strings.Index,
}

// Count checker verifies that obtained value has the specified number of non-overlapping instances of sep
var Count check.Checker = &substringCountChecker{
	&check.CheckerInfo{
		Name:   "Count",
		Params: []string{"obtained", "sep", "expected"},
	},
	strings.Count,
}

type substringCountChecker struct {
	*check.CheckerInfo
	substringFunction func(string, string) int
}

func (checker *substringCountChecker) Check(params []interface{}, names []string) (bool, string) {
	obtained := params[0]
	substring := params[1]
	expected := params[2]
	substringStr, ok := substring.(string)
	if !ok {
		return false, fmt.Sprintf("%s value must be a string.", names[1])
	}
	obtainedString, obtainedIsStr := obtained.(string)
	if !obtainedIsStr {
		if obtainedWithStringer, obtainedHasStringer := obtained.(fmt.Stringer); obtainedHasStringer {
			obtainedString, obtainedIsStr = obtainedWithStringer.String(), true
		}
	}
	if obtainedIsStr {
		return checker.substringFunction(obtainedString, substringStr) == expected, ""
	}
	return false, "obtained value is not a string and has no .String()."
}

// IsLower checker verifies that the obtained value is in lower case
var IsLower check.Checker = &stringTransformChecker{
	&check.CheckerInfo{
		Name:   "IsLower",
		Params: []string{"obtained"},
	},
	strings.ToLower,
}

// IsUpper checker verifies that the obtained value is in lower case
var IsUpper check.Checker = &stringTransformChecker{
	&check.CheckerInfo{
		Name:   "IsUpper",
		Params: []string{"obtained"},
	},
	strings.ToUpper,
}

type stringTransformChecker struct {
	*check.CheckerInfo
	stringFunction func(string) string
}

func (checker *stringTransformChecker) Check(params []interface{}, names []string) (bool, string) {
	obtained := params[0]
	obtainedString, obtainedIsStr := obtained.(string)
	if !obtainedIsStr {
		if obtainedWithStringer, obtainedHasStringer := obtained.(fmt.Stringer); obtainedHasStringer {
			obtainedString, obtainedIsStr = obtainedWithStringer.String(), true
		}
	}
	if obtainedIsStr {
		return checker.stringFunction(obtainedString) == obtainedString, ""
	}
	return false, "obtained value is not a string and has no .String()."
}
