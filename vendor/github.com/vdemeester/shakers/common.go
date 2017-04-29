package shakers

import (
	"reflect"
	"time"

	"github.com/go-check/check"
)

// As a commodity, we bring all check.Checker variables into the current namespace to avoid having
// to think about check.X versus checker.X.
var (
	DeepEquals   = check.DeepEquals
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

// Equaler is an interface implemented if the type has a Equal method.
// This is used to compare struct using shakers.Equals.
type Equaler interface {
	Equal(Equaler) bool
}

// Equals checker verifies the obtained value is equal to the specified one.
// It's is smart in a wait that it supports several *types* (built-in, Equaler,
// time.Time)
//
//    c.Assert(myStruct, Equals, aStruct, check.Commentf("bouuuhh"))
//    c.Assert(myTime, Equals, aTime, check.Commentf("bouuuhh"))
//
var Equals check.Checker = &equalChecker{
	&check.CheckerInfo{
		Name:   "Equals",
		Params: []string{"obtained", "expected"},
	},
}

type equalChecker struct {
	*check.CheckerInfo
}

func (checker *equalChecker) Check(params []interface{}, names []string) (bool, string) {
	return isEqual(params[0], params[1])
}

func isEqual(obtained, expected interface{}) (bool, string) {
	switch obtained.(type) {
	case time.Time:
		return timeEquals(obtained, expected)
	case Equaler:
		return equalerEquals(obtained, expected)
	default:
		if reflect.TypeOf(obtained) != reflect.TypeOf(expected) {
			return false, "obtained value and expected value have not the same type."
		}
		return obtained == expected, ""
	}
}

func equalerEquals(obtained, expected interface{}) (bool, string) {
	expectedEqualer, ok := expected.(Equaler)
	if !ok {
		return false, "expected value must be an Equaler - implementing Equal(Equaler)."
	}
	obtainedEqualer, ok := obtained.(Equaler)
	if !ok {
		return false, "obtained value must be an Equaler - implementing Equal(Equaler)."
	}
	return obtainedEqualer.Equal(expectedEqualer), ""
}

// GreaterThan checker verifies the obtained value is greater than the specified one.
// It's is smart in a wait that it supports several *types* (built-in, time.Time)
//
//    c.Assert(myTime, GreaterThan, aTime, check.Commentf("bouuuhh"))
//    c.Assert(myInt, GreaterThan, 2, check.Commentf("bouuuhh"))
//
var GreaterThan check.Checker = &greaterThanChecker{
	&check.CheckerInfo{
		Name:   "GreaterThan",
		Params: []string{"obtained", "expected"},
	},
}

type greaterThanChecker struct {
	*check.CheckerInfo
}

func (checker *greaterThanChecker) Check(params []interface{}, names []string) (bool, string) {
	return greaterThan(params[0], params[1])
}

func greaterThan(obtained, expected interface{}) (bool, string) {
	if _, ok := obtained.(time.Time); ok {
		return isAfter(obtained, expected)
	}
	if reflect.TypeOf(obtained) != reflect.TypeOf(expected) {
		return false, "obtained value and expected value have not the same type."
	}
	switch v := obtained.(type) {
	case float32:
		return v > expected.(float32), ""
	case float64:
		return v > expected.(float64), ""
	case int:
		return v > expected.(int), ""
	case int8:
		return v > expected.(int8), ""
	case int16:
		return v > expected.(int16), ""
	case int32:
		return v > expected.(int32), ""
	case int64:
		return v > expected.(int64), ""
	case uint:
		return v > expected.(uint), ""
	case uint8:
		return v > expected.(uint8), ""
	case uint16:
		return v > expected.(uint16), ""
	case uint32:
		return v > expected.(uint32), ""
	case uint64:
		return v > expected.(uint64), ""
	default:
		return false, "obtained value type not supported."
	}
}

// GreaterOrEqualThan checker verifies the obtained value is greater or equal than the specified one.
// It's is smart in a wait that it supports several *types* (built-in, time.Time)
//
//    c.Assert(myTime, GreaterOrEqualThan, aTime, check.Commentf("bouuuhh"))
//    c.Assert(myInt, GreaterOrEqualThan, 2, check.Commentf("bouuuhh"))
//
var GreaterOrEqualThan check.Checker = &greaterOrEqualThanChecker{
	&check.CheckerInfo{
		Name:   "GreaterOrEqualThan",
		Params: []string{"obtained", "expected"},
	},
}

type greaterOrEqualThanChecker struct {
	*check.CheckerInfo
}

func (checker *greaterOrEqualThanChecker) Check(params []interface{}, names []string) (bool, string) {
	return greaterOrEqualThan(params[0], params[1])
}

func greaterOrEqualThan(obtained, expected interface{}) (bool, string) {
	if _, ok := obtained.(time.Time); ok {
		return isAfter(obtained, expected)
	}
	if reflect.TypeOf(obtained) != reflect.TypeOf(expected) {
		return false, "obtained value and expected value have not the same type."
	}
	switch v := obtained.(type) {
	case float32:
		return v >= expected.(float32), ""
	case float64:
		return v >= expected.(float64), ""
	case int:
		return v >= expected.(int), ""
	case int8:
		return v >= expected.(int8), ""
	case int16:
		return v >= expected.(int16), ""
	case int32:
		return v >= expected.(int32), ""
	case int64:
		return v >= expected.(int64), ""
	case uint:
		return v >= expected.(uint), ""
	case uint8:
		return v >= expected.(uint8), ""
	case uint16:
		return v >= expected.(uint16), ""
	case uint32:
		return v >= expected.(uint32), ""
	case uint64:
		return v >= expected.(uint64), ""
	default:
		return false, "obtained value type not supported."
	}
}

// LessThan checker verifies the obtained value is less than the specified one.
// It's is smart in a wait that it supports several *types* (built-in, time.Time)
//
//    c.Assert(myTime, LessThan, aTime, check.Commentf("bouuuhh"))
//    c.Assert(myInt, LessThan, 2, check.Commentf("bouuuhh"))
//
var LessThan check.Checker = &lessThanChecker{
	&check.CheckerInfo{
		Name:   "LessThan",
		Params: []string{"obtained", "expected"},
	},
}

type lessThanChecker struct {
	*check.CheckerInfo
}

func (checker *lessThanChecker) Check(params []interface{}, names []string) (bool, string) {
	return lessThan(params[0], params[1])
}

func lessThan(obtained, expected interface{}) (bool, string) {
	if _, ok := obtained.(time.Time); ok {
		return isBefore(obtained, expected)
	}
	if reflect.TypeOf(obtained) != reflect.TypeOf(expected) {
		return false, "obtained value and expected value have not the same type."
	}
	switch v := obtained.(type) {
	case float32:
		return v < expected.(float32), ""
	case float64:
		return v < expected.(float64), ""
	case int:
		return v < expected.(int), ""
	case int8:
		return v < expected.(int8), ""
	case int16:
		return v < expected.(int16), ""
	case int32:
		return v < expected.(int32), ""
	case int64:
		return v < expected.(int64), ""
	case uint:
		return v < expected.(uint), ""
	case uint8:
		return v < expected.(uint8), ""
	case uint16:
		return v < expected.(uint16), ""
	case uint32:
		return v < expected.(uint32), ""
	case uint64:
		return v < expected.(uint64), ""
	default:
		return false, "obtained value type not supported."
	}
}

// LessOrEqualThan checker verifies the obtained value is less or equal than the specified one.
// It's is smart in a wait that it supports several *types* (built-in, time.Time)
//
//    c.Assert(myTime, LessThan, aTime, check.Commentf("bouuuhh"))
//    c.Assert(myInt, LessThan, 2, check.Commentf("bouuuhh"))
//
var LessOrEqualThan check.Checker = &lessOrEqualThanChecker{
	&check.CheckerInfo{
		Name:   "LessOrEqualThan",
		Params: []string{"obtained", "expected"},
	},
}

type lessOrEqualThanChecker struct {
	*check.CheckerInfo
}

func (checker *lessOrEqualThanChecker) Check(params []interface{}, names []string) (bool, string) {
	return lessOrEqualThan(params[0], params[1])
}

func lessOrEqualThan(obtained, expected interface{}) (bool, string) {
	if _, ok := obtained.(time.Time); ok {
		return isBefore(obtained, expected)
	}
	if reflect.TypeOf(obtained) != reflect.TypeOf(expected) {
		return false, "obtained value and expected value have not the same type."
	}
	switch v := obtained.(type) {
	case float32:
		return v <= expected.(float32), ""
	case float64:
		return v <= expected.(float64), ""
	case int:
		return v <= expected.(int), ""
	case int8:
		return v <= expected.(int8), ""
	case int16:
		return v <= expected.(int16), ""
	case int32:
		return v <= expected.(int32), ""
	case int64:
		return v <= expected.(int64), ""
	case uint:
		return v <= expected.(uint), ""
	case uint8:
		return v <= expected.(uint8), ""
	case uint16:
		return v <= expected.(uint16), ""
	case uint32:
		return v <= expected.(uint32), ""
	case uint64:
		return v <= expected.(uint64), ""
	default:
		return false, "obtained value type not supported."
	}
}
