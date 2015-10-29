package shakers

import (
	"fmt"
	"time"

	"github.com/go-check/check"
)

// Default format when parsing (in addition to RFC and default time formats..)
const shortForm = "2006-01-02"

// IsBefore checker verifies the specified value is before the specified time.
// It is exclusive.
//
//    c.Assert(myTime, IsBefore, theTime, check.Commentf("bouuuhhh"))
//
var IsBefore check.Checker = &isBeforeChecker{
	&check.CheckerInfo{
		Name:   "IsBefore",
		Params: []string{"obtained", "expected"},
	},
}

type isBeforeChecker struct {
	*check.CheckerInfo
}

func (checker *isBeforeChecker) Check(params []interface{}, names []string) (bool, string) {
	return isBefore(params[0], params[1])
}

func isBefore(value, t interface{}) (bool, string) {
	tTime, ok := parseTime(t)
	if !ok {
		return false, "expected must be a Time struct, or parseable."
	}
	valueTime, valueIsTime := parseTime(value)
	if valueIsTime {
		return valueTime.Before(tTime), ""
	}
	return false, "obtained value is not a time.Time struct or parseable as a time."
}

// IsAfter checker verifies the specified value is before the specified time.
// It is exclusive.
//
//    c.Assert(myTime, IsAfter, theTime, check.Commentf("bouuuhhh"))
//
var IsAfter check.Checker = &isAfterChecker{
	&check.CheckerInfo{
		Name:   "IsAfter",
		Params: []string{"obtained", "expected"},
	},
}

type isAfterChecker struct {
	*check.CheckerInfo
}

func (checker *isAfterChecker) Check(params []interface{}, names []string) (bool, string) {
	return isAfter(params[0], params[1])
}

func isAfter(value, t interface{}) (bool, string) {
	tTime, ok := parseTime(t)
	if !ok {
		return false, "expected must be a Time struct, or parseable."
	}
	valueTime, valueIsTime := parseTime(value)
	if valueIsTime {
		return valueTime.After(tTime), ""
	}
	return false, "obtained value is not a time.Time struct or parseable as a time."
}

// IsBetween checker verifies the specified time is between the specified start
// and end. It's exclusive so if the specified time is at the tip of the interval.
//
//    c.Assert(myTime, IsBetween, startTime, endTime, check.Commentf("bouuuhhh"))
//
var IsBetween check.Checker = &isBetweenChecker{
	&check.CheckerInfo{
		Name:   "IsBetween",
		Params: []string{"obtained", "start", "end"},
	},
}

type isBetweenChecker struct {
	*check.CheckerInfo
}

func (checker *isBetweenChecker) Check(params []interface{}, names []string) (bool, string) {
	return isBetween(params[0], params[1], params[2])
}

func isBetween(value, start, end interface{}) (bool, string) {
	startTime, ok := parseTime(start)
	if !ok {
		return false, "start must be a Time struct, or parseable."
	}
	endTime, ok := parseTime(end)
	if !ok {
		return false, "end must be a Time struct, or parseable."
	}
	valueTime, valueIsTime := parseTime(value)
	if valueIsTime {
		return valueTime.After(startTime) && valueTime.Before(endTime), ""
	}
	return false, "obtained value is not a time.Time struct or parseable as a time."
}

// TimeEquals checker verifies the specified time is the equal to the expected
// time.
//
//    c.Assert(myTime, TimeEquals, expected, check.Commentf("bouhhh"))
//
// It's possible to ignore some part of the time (like hours, minutes, etc..) using
// the TimeIgnore checker with it.
//
//    c.Assert(myTime, TimeIgnore(TimeEquals, time.Hour), expected, check.Commentf("... bouh.."))
//
var TimeEquals check.Checker = &timeEqualsChecker{
	&check.CheckerInfo{
		Name:   "TimeEquals",
		Params: []string{"obtained", "expected"},
	},
}

type timeEqualsChecker struct {
	*check.CheckerInfo
}

func (checker *timeEqualsChecker) Check(params []interface{}, names []string) (bool, string) {
	return timeEquals(params[0], params[1])
}

func timeEquals(obtained, expected interface{}) (bool, string) {
	expectedTime, ok := parseTime(expected)
	if !ok {
		return false, "expected must be a Time struct, or parseable."
	}
	valueTime, valueIsTime := parseTime(obtained)
	if valueIsTime {
		return valueTime.Equal(expectedTime), ""
	}
	return false, "obtained value is not a time.Time struct or parseable as a time."
}

// TimeIgnore checker will ignore some part of the time on the encapsulated checker.
//
//    c.Assert(myTime, TimeIgnore(IsBetween, time.Second), start, end)
//
// FIXME use interface{} for ignore (to enable "Month", ..
func TimeIgnore(checker check.Checker, ignore time.Duration) check.Checker {
	return &timeIgnoreChecker{
		sub:    checker,
		ignore: ignore,
	}
}

type timeIgnoreChecker struct {
	sub    check.Checker
	ignore time.Duration
}

func (checker *timeIgnoreChecker) Info() *check.CheckerInfo {
	info := *checker.sub.Info()
	info.Name = fmt.Sprintf("TimeIgnore(%s, %v)", info.Name, checker.ignore)
	return &info
}

func (checker *timeIgnoreChecker) Check(params []interface{}, names []string) (bool, string) {
	// Naive implementation : all params are supposed to be date
	mParams := make([]interface{}, len(params))
	for index, param := range params {
		paramTime, ok := parseTime(param)
		if !ok {
			return false, fmt.Sprintf("%s must be a Time struct, or parseable.", names[index])
		}
		year := paramTime.Year()
		month := paramTime.Month()
		day := paramTime.Day()
		hour := paramTime.Hour()
		min := paramTime.Minute()
		sec := paramTime.Second()
		nsec := paramTime.Nanosecond()
		location := paramTime.Location()
		switch checker.ignore {
		case time.Hour:
			hour = 0
			fallthrough
		case time.Minute:
			min = 0
			fallthrough
		case time.Second:
			sec = 0
			fallthrough
		case time.Millisecond:
			fallthrough
		case time.Microsecond:
			fallthrough
		case time.Nanosecond:
			nsec = 0
		}
		mParams[index] = time.Date(year, month, day, hour, min, sec, nsec, location)
	}
	return checker.sub.Check(mParams, names)
}

func parseTime(datetime interface{}) (time.Time, bool) {
	switch datetime.(type) {
	case time.Time:
		return datetime.(time.Time), true
	case string:
		return parseTimeAsString(datetime.(string))
	default:
		if datetimeWithStr, ok := datetime.(fmt.Stringer); ok {
			return parseTimeAsString(datetimeWithStr.String())
		}
		return time.Time{}, false
	}
}

func parseTimeAsString(timeAsStr string) (time.Time, bool) {
	forms := []string{shortForm, time.RFC3339, time.RFC3339Nano, time.RFC822, time.RFC822Z}
	for _, form := range forms {
		datetime, err := time.Parse(form, timeAsStr)
		if err == nil {
			return datetime, true
		}
	}
	return time.Time{}, false
}
