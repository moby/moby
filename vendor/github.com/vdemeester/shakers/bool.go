package shakers

import (
	"github.com/go-check/check"
)

// True checker verifies the obtained value is true
//
//    c.Assert(myBool, True)
//
var True check.Checker = &boolChecker{
	&check.CheckerInfo{
		Name:   "True",
		Params: []string{"obtained"},
	},
	true,
}

// False checker verifies the obtained value is false
//
//    c.Assert(myBool, False)
//
var False check.Checker = &boolChecker{
	&check.CheckerInfo{
		Name:   "False",
		Params: []string{"obtained"},
	},
	false,
}

type boolChecker struct {
	*check.CheckerInfo
	expected bool
}

func (checker *boolChecker) Check(params []interface{}, names []string) (bool, string) {
	return is(checker.expected, params[0])
}

func is(expected bool, obtained interface{}) (bool, string) {
	obtainedBool, ok := obtained.(bool)
	if !ok {
		return false, "obtained value must be a bool."
	}
	return obtainedBool == expected, ""
}
